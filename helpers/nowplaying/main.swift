import Foundation

// MediaRemote is a private framework that provides system-wide Now Playing info
// This works with ANY audio source on macOS (browsers, apps, etc.)

@objc protocol MRMediaRemoteGetNowPlayingInfoProtocol {
    func MRMediaRemoteGetNowPlayingInfo(_ queue: DispatchQueue, _ completion: @escaping ([String: Any]) -> Void)
}

// Dynamic loading of MediaRemote framework
let bundle = CFBundleCreate(kCFAllocatorDefault, NSURL(fileURLWithPath: "/System/Library/PrivateFrameworks/MediaRemote.framework"))

// Function pointers
typealias MRMediaRemoteGetNowPlayingInfoType = @convention(c) (DispatchQueue, @escaping ([String: Any]) -> Void) -> Void
typealias MRMediaRemoteGetNowPlayingApplicationIsPlayingType = @convention(c) (DispatchQueue, @escaping (Bool) -> Void) -> Void
typealias MRMediaRemoteSendCommandType = @convention(c) (UInt32, UnsafeMutableRawPointer?) -> Bool

let MRMediaRemoteGetNowPlayingInfo = unsafeBitCast(
    CFBundleGetFunctionPointerForName(bundle, "MRMediaRemoteGetNowPlayingInfo" as CFString),
    to: MRMediaRemoteGetNowPlayingInfoType.self
)

let MRMediaRemoteGetNowPlayingApplicationIsPlaying = unsafeBitCast(
    CFBundleGetFunctionPointerForName(bundle, "MRMediaRemoteGetNowPlayingApplicationIsPlaying" as CFString),
    to: MRMediaRemoteGetNowPlayingApplicationIsPlayingType.self
)

let MRMediaRemoteSendCommand = unsafeBitCast(
    CFBundleGetFunctionPointerForName(bundle, "MRMediaRemoteSendCommand" as CFString),
    to: MRMediaRemoteSendCommandType.self
)

// MediaRemote command constants
enum MRCommand: UInt32 {
    case play = 0
    case pause = 1
    case togglePlayPause = 2
    case stop = 3
    case nextTrack = 4
    case previousTrack = 5
}

// MediaRemote info keys
let kMRMediaRemoteNowPlayingInfoTitle = "kMRMediaRemoteNowPlayingInfoTitle"
let kMRMediaRemoteNowPlayingInfoArtist = "kMRMediaRemoteNowPlayingInfoArtist"
let kMRMediaRemoteNowPlayingInfoAlbum = "kMRMediaRemoteNowPlayingInfoAlbum"
let kMRMediaRemoteNowPlayingInfoDuration = "kMRMediaRemoteNowPlayingInfoDuration"
let kMRMediaRemoteNowPlayingInfoElapsedTime = "kMRMediaRemoteNowPlayingInfoElapsedTime"

func getMetadata() {
    let semaphore = DispatchSemaphore(value: 0)

    MRMediaRemoteGetNowPlayingInfo(DispatchQueue.main) { info in
        let title = info[kMRMediaRemoteNowPlayingInfoTitle] as? String ?? ""
        let artist = info[kMRMediaRemoteNowPlayingInfoArtist] as? String ?? ""
        let album = info[kMRMediaRemoteNowPlayingInfoAlbum] as? String ?? ""

        MRMediaRemoteGetNowPlayingApplicationIsPlaying(DispatchQueue.main) { isPlaying in
            let status = isPlaying ? "playing" : "paused"
            print("\(title)|\(artist)|\(album)|\(status)")
            semaphore.signal()
        }
    }

    semaphore.wait()
}

func getDuration() {
    let semaphore = DispatchSemaphore(value: 0)

    MRMediaRemoteGetNowPlayingInfo(DispatchQueue.main) { info in
        let duration = info[kMRMediaRemoteNowPlayingInfoDuration] as? Double ?? 0
        print(Int(duration))
        semaphore.signal()
    }

    semaphore.wait()
}

func getPosition() {
    let semaphore = DispatchSemaphore(value: 0)

    MRMediaRemoteGetNowPlayingInfo(DispatchQueue.main) { info in
        let position = info[kMRMediaRemoteNowPlayingInfoElapsedTime] as? Double ?? 0
        print(position)
        semaphore.signal()
    }

    semaphore.wait()
}

func sendCommand(_ command: MRCommand) {
    _ = MRMediaRemoteSendCommand(command.rawValue, nil)
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
