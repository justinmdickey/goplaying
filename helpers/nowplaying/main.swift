import Foundation

// MediaRemote is a private framework that provides system-wide Now Playing info
// This works with ANY audio source on macOS (browsers, apps, etc.)

// Dynamic loading of MediaRemote framework
guard let bundle = CFBundleCreate(kCFAllocatorDefault, NSURL(fileURLWithPath: "/System/Library/PrivateFrameworks/MediaRemote.framework")) else {
    fputs("Error: Could not load MediaRemote framework\n", stderr)
    exit(1)
}

// Function pointers
typealias MRMediaRemoteGetNowPlayingInfoType = @convention(c) (DispatchQueue, @escaping ([String: Any]) -> Void) -> Void
typealias MRMediaRemoteGetNowPlayingApplicationIsPlayingType = @convention(c) (DispatchQueue, @escaping (Bool) -> Void) -> Void
typealias MRMediaRemoteSendCommandType = @convention(c) (UInt32, UnsafeMutableRawPointer?) -> Bool

guard let getNowPlayingInfoPtr = CFBundleGetFunctionPointerForName(bundle, "MRMediaRemoteGetNowPlayingInfo" as CFString) else {
    fputs("Error: Could not find MRMediaRemoteGetNowPlayingInfo\n", stderr)
    exit(1)
}

guard let getIsPlayingPtr = CFBundleGetFunctionPointerForName(bundle, "MRMediaRemoteGetNowPlayingApplicationIsPlaying" as CFString) else {
    fputs("Error: Could not find MRMediaRemoteGetNowPlayingApplicationIsPlaying\n", stderr)
    exit(1)
}

guard let sendCommandPtr = CFBundleGetFunctionPointerForName(bundle, "MRMediaRemoteSendCommand" as CFString) else {
    fputs("Error: Could not find MRMediaRemoteSendCommand\n", stderr)
    exit(1)
}

let MRMediaRemoteGetNowPlayingInfo = unsafeBitCast(getNowPlayingInfoPtr, to: MRMediaRemoteGetNowPlayingInfoType.self)
let MRMediaRemoteGetNowPlayingApplicationIsPlaying = unsafeBitCast(getIsPlayingPtr, to: MRMediaRemoteGetNowPlayingApplicationIsPlayingType.self)
let MRMediaRemoteSendCommand = unsafeBitCast(sendCommandPtr, to: MRMediaRemoteSendCommandType.self)

// MediaRemote command constants
enum MRCommand: UInt32 {
    case play = 0
    case pause = 1
    case togglePlayPause = 2
    case stop = 3
    case nextTrack = 4
    case previousTrack = 5
}

func getMetadata() {
    let semaphore = DispatchSemaphore(value: 0)
    var hasInfo = false

    MRMediaRemoteGetNowPlayingInfo(DispatchQueue.main) { info in
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
            if keyStr.contains("Title") {
                title = info[key] as? String ?? title
            } else if keyStr.contains("Artist") {
                artist = info[key] as? String ?? artist
            } else if keyStr.contains("Album") {
                album = info[key] as? String ?? album
            }
        }

        MRMediaRemoteGetNowPlayingApplicationIsPlaying(DispatchQueue.main) { isPlaying in
            let status = isPlaying ? "playing" : "paused"

            // Only output if we have at least a title
            if !title.isEmpty {
                print("\(title)|\(artist)|\(album)|\(status)")
                hasInfo = true
            }

            semaphore.signal()
        }
    }

    _ = semaphore.wait(timeout: .now() + 2)

    if !hasInfo {
        fputs("Error: No media playing\n", stderr)
        exit(1)
    }
}

func getDuration() {
    let semaphore = DispatchSemaphore(value: 0)

    MRMediaRemoteGetNowPlayingInfo(DispatchQueue.main) { info in
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

    _ = semaphore.wait(timeout: .now() + 2)
}

func getPosition() {
    let semaphore = DispatchSemaphore(value: 0)

    MRMediaRemoteGetNowPlayingInfo(DispatchQueue.main) { info in
        var position: Double = 0

        // Try to find elapsed time with different key formats
        for key in info.keys {
            let keyStr = String(describing: key)
            if keyStr.contains("Elapsed") || keyStr.contains("Position") {
                position = info[key] as? Double ?? position
                break
            }
        }

        print(position)
        semaphore.signal()
    }

    _ = semaphore.wait(timeout: .now() + 2)
}

func sendCommand(_ command: MRCommand) {
    _ = MRMediaRemoteSendCommand(command.rawValue, nil)
    // Give command time to execute
    usleep(100000) // 100ms
}

// Main
guard CommandLine.arguments.count > 1 else {
    fputs("Usage: nowplaying <command>\n", stderr)
    fputs("Commands: metadata, duration, position, play-pause, next, previous\n", stderr)
    exit(1)
}

let command = CommandLine.arguments[1]

switch command {
case "metadata":
    getMetadata()
case "duration":
    getDuration()
case "position":
    getPosition()
case "play-pause":
    sendCommand(.togglePlayPause)
case "next":
    sendCommand(.nextTrack)
case "previous":
    sendCommand(.previousTrack)
default:
    fputs("Unknown command: \(command)\n", stderr)
    exit(1)
}
