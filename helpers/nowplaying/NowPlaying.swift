import Foundation

// MediaRemote is a private framework that provides system-wide Now Playing info
// This works with ANY audio source on macOS (browsers, apps, etc.)
//
// Since macOS 15.4, mediaremoted only returns Now Playing info to Apple-signed
// clients; any other process gets an empty dictionary. So this is built as a
// dylib and loaded into /usr/bin/perl (an Apple platform binary that is still
// allowed), which calls nowplaying_main as an XSUB:
//
//   /usr/bin/perl -e '<loader>' libnowplaying.dylib <command>
//
// See runHelper in media_darwin.go for the loader.

// Function pointers
typealias MRMediaRemoteGetNowPlayingInfoType = @convention(c) (DispatchQueue, @escaping ([String: Any]) -> Void) -> Void
typealias MRMediaRemoteGetNowPlayingApplicationIsPlayingType = @convention(c) (DispatchQueue, @escaping (Bool) -> Void) -> Void
typealias MRMediaRemoteSendCommandType = @convention(c) (UInt32, UnsafeMutableRawPointer?) -> Bool

var MRMediaRemoteGetNowPlayingInfo: MRMediaRemoteGetNowPlayingInfoType!
var MRMediaRemoteGetNowPlayingApplicationIsPlaying: MRMediaRemoteGetNowPlayingApplicationIsPlayingType!
var MRMediaRemoteSendCommand: MRMediaRemoteSendCommandType!

// Dynamic loading of MediaRemote framework
func loadMediaRemote() {
    guard let bundle = CFBundleCreate(kCFAllocatorDefault, NSURL(fileURLWithPath: "/System/Library/PrivateFrameworks/MediaRemote.framework")) else {
        fail("Error: Could not load MediaRemote framework")
    }

    func symbol(_ name: String) -> UnsafeMutableRawPointer {
        guard let ptr = CFBundleGetFunctionPointerForName(bundle, name as CFString) else {
            fail("Error: Could not find \(name)")
        }
        return ptr
    }

    MRMediaRemoteGetNowPlayingInfo = unsafeBitCast(symbol("MRMediaRemoteGetNowPlayingInfo"), to: MRMediaRemoteGetNowPlayingInfoType.self)
    MRMediaRemoteGetNowPlayingApplicationIsPlaying = unsafeBitCast(symbol("MRMediaRemoteGetNowPlayingApplicationIsPlaying"), to: MRMediaRemoteGetNowPlayingApplicationIsPlayingType.self)
    MRMediaRemoteSendCommand = unsafeBitCast(symbol("MRMediaRemoteSendCommand"), to: MRMediaRemoteSendCommandType.self)
}

// MediaRemote delivers its callbacks on the queue we pass in. The host
// process has no run loop, so DispatchQueue.main would never drain while the
// main thread is blocked on the semaphore below and every call would time
// out. A dedicated serial queue lets the callbacks run on a worker thread.
let nowPlayingQueue = DispatchQueue(label: "goplaying.nowplaying")

// MediaRemote command constants
enum MRCommand: UInt32 {
    case play = 0
    case pause = 1
    case togglePlayPause = 2
    case stop = 3
    case nextTrack = 4
    case previousTrack = 5
}

// Exits the host process. Flushes stdout first since the perl host never
// returns through Swift's normal teardown.
func fail(_ message: String) -> Never {
    fflush(stdout)
    fputs(message + "\n", stderr)
    exit(1)
}

func getMetadata() {
    let semaphore = DispatchSemaphore(value: 0)
    var hasInfo = false
    var infoCount = -1

    MRMediaRemoteGetNowPlayingInfo(nowPlayingQueue) { info in
        infoCount = info.count
        // Check if we have any info at all
        if info.isEmpty {
            semaphore.signal()
            return
        }

        // Try different key formats - macOS versions use different keys
        var title = ""
        var artist = ""
        var album = ""

        // Try both old and new key formats
        for key in info.keys {
            let keyStr = String(describing: key)
            // Try both old and new key formats with priority, avoid accidental matches
            if keyStr == "kMRMediaRemoteNowPlayingInfoTitle" || (keyStr.hasSuffix("Title") && !keyStr.contains("Album")) {
                title = info[key] as? String ?? title
            } else if keyStr == "kMRMediaRemoteNowPlayingInfoArtist" || keyStr.hasSuffix("Artist") {
                artist = info[key] as? String ?? artist
            } else if keyStr == "kMRMediaRemoteNowPlayingInfoAlbum" || keyStr.hasSuffix("Album") {
                album = info[key] as? String ?? album
            }
        }

        MRMediaRemoteGetNowPlayingApplicationIsPlaying(nowPlayingQueue) { isPlaying in
            let status = isPlaying ? "playing" : "paused"

            // Only output if we have at least a title
            if !title.isEmpty {
                print("\(title)|\(artist)|\(album)|\(status)")
                hasInfo = true
            }

            semaphore.signal()
        }
    }

    // Safety net for a slow callback (e.g. a cold mediaremoted daemon on the
    // first call). When there is genuinely nothing playing the callback fires
    // immediately with empty info, so this value adds no idle-state latency.
    let result = semaphore.wait(timeout: .now() + 1.0)

    if result == .timedOut {
        fail("Error: Timeout waiting for Now Playing info")
    }

    if !hasInfo {
        // Distinguish "daemon returned nothing" (count 0) from "info came back
        // but the keys didn't match" (count > 0) for future macOS versions.
        fail("Error: No media playing (info keys: \(infoCount))")
    }
}

func getDuration() {
    let semaphore = DispatchSemaphore(value: 0)

    MRMediaRemoteGetNowPlayingInfo(nowPlayingQueue) { info in
        var duration: Double = 0

        // Try to find duration with different key formats
        for key in info.keys {
            let keyStr = String(describing: key)
            if keyStr.contains("Duration") {
                duration = info[key] as? Double ?? duration
                break
            }
        }

        print(Int(duration))
        semaphore.signal()
    }

    // Safety net for a slow callback (see getMetadata)
    let result = semaphore.wait(timeout: .now() + 1.0)
    if result == .timedOut {
        fputs("Warning: Timeout getting duration\n", stderr)
    }
}

func getPosition() {
    let semaphore = DispatchSemaphore(value: 0)

    MRMediaRemoteGetNowPlayingInfo(nowPlayingQueue) { info in
        var position: Double = 0
        var timestamp: Date?
        var rate: Double = 0

        // Try to find elapsed time with different key formats
        for key in info.keys {
            let keyStr = String(describing: key)
            if keyStr.contains("Elapsed") || keyStr.contains("Position") {
                position = info[key] as? Double ?? position
            } else if keyStr.hasSuffix("Timestamp") {
                timestamp = info[key] as? Date
            } else if keyStr.hasSuffix("PlaybackRate") {
                rate = info[key] as? Double ?? rate
            }
        }

        // Elapsed time is a snapshot taken at Timestamp; advance it by the
        // time since then at the current playback rate (0 when paused).
        if let timestamp = timestamp {
            position += Date().timeIntervalSince(timestamp) * rate
        }

        print(position)
        semaphore.signal()
    }

    // Safety net for a slow callback (see getMetadata)
    let result = semaphore.wait(timeout: .now() + 1.0)
    if result == .timedOut {
        fputs("Warning: Timeout getting position\n", stderr)
    }
}

func getArtwork() {
    let semaphore = DispatchSemaphore(value: 0)

    MRMediaRemoteGetNowPlayingInfo(nowPlayingQueue) { info in
        // Try to find artwork data with different key formats
        for key in info.keys {
            let keyStr = String(describing: key)
            if keyStr.contains("Artwork") && !keyStr.contains("Identifier") {
                if let artworkData = info[key] as? Data {
                    // Output base64-encoded artwork data
                    let base64String = artworkData.base64EncodedString()
                    print(base64String)
                    semaphore.signal()
                    return
                }
            }
        }

        // No artwork found
        semaphore.signal()
    }

    // Safety net for a slow callback (see getMetadata)
    let result = semaphore.wait(timeout: .now() + 1.0)
    if result == .timedOut {
        fputs("Warning: Timeout getting artwork\n", stderr)
    }
}

func sendCommand(_ command: MRCommand) {
    _ = MRMediaRemoteSendCommand(command.rawValue, nil)
    // Brief delay to ensure MediaRemote command is processed before returning.
    // MediaRemote sends commands asynchronously to the target app, and returning
    // too quickly can cause the app to miss rapid successive commands.
    // 100ms provides reliable command delivery without noticeable delay to users.
    usleep(100000) // 100ms
}

// Entry point, installed by the perl loader as an XSUB. The perl interpreter
// and CV arguments are unused; the command is the host's last argv entry.
@_cdecl("nowplaying_main")
public func nowplayingMain(_ perl: UnsafeMutableRawPointer?, _ cv: UnsafeMutableRawPointer?) {
    let args = ProcessInfo.processInfo.arguments
    guard args.count > 1, let command = args.last else {
        fputs("Usage: perl -e '<loader>' libnowplaying.dylib <command>\n", stderr)
        fail("Commands: metadata, duration, position, artwork, play-pause, next, previous")
    }

    loadMediaRemote()

    switch command {
    case "metadata":
        getMetadata()
    case "duration":
        getDuration()
    case "position":
        getPosition()
    case "artwork":
        getArtwork()
    case "play-pause":
        sendCommand(.togglePlayPause)
    case "next":
        sendCommand(.nextTrack)
    case "previous":
        sendCommand(.previousTrack)
    default:
        fail("Unknown command: \(command)")
    }

    fflush(stdout)
}
