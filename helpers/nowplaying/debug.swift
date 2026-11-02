// MediaRemote diagnostics: dumps the raw Now Playing info dictionary so we
// can see exactly what the current macOS version returns.
//
// Usage:
//   swift debug.swift
// or
//   swiftc debug.swift -o mr-debug && ./mr-debug
import Foundation

print("macOS: \(ProcessInfo.processInfo.operatingSystemVersionString)")

guard let bundle = CFBundleCreate(kCFAllocatorDefault, NSURL(fileURLWithPath: "/System/Library/PrivateFrameworks/MediaRemote.framework")) else {
    print("FAIL: could not load /System/Library/PrivateFrameworks/MediaRemote.framework")
    exit(1)
}
print("OK: framework loaded")

typealias GetInfoType = @convention(c) (DispatchQueue, @escaping ([String: Any]) -> Void) -> Void
typealias GetIsPlayingType = @convention(c) (DispatchQueue, @escaping (Bool) -> Void) -> Void

guard let infoPtr = CFBundleGetFunctionPointerForName(bundle, "MRMediaRemoteGetNowPlayingInfo" as CFString) else {
    print("FAIL: symbol MRMediaRemoteGetNowPlayingInfo not found")
    exit(1)
}
guard let isPlayingPtr = CFBundleGetFunctionPointerForName(bundle, "MRMediaRemoteGetNowPlayingApplicationIsPlaying" as CFString) else {
    print("FAIL: symbol MRMediaRemoteGetNowPlayingApplicationIsPlaying not found")
    exit(1)
}
print("OK: symbols resolved")

let getInfo = unsafeBitCast(infoPtr, to: GetInfoType.self)
let getIsPlaying = unsafeBitCast(isPlayingPtr, to: GetIsPlayingType.self)

// Dedicated serial queue: this script's main thread blocks on the semaphore,
// so callbacks must run on a worker thread (DispatchQueue.main would never
// drain and we'd time out).
let queue = DispatchQueue(label: "goplaying.debug")
let sem = DispatchSemaphore(value: 0)

getInfo(queue) { info in
    print("INFO COUNT: \(info.count)")
    let keys = info.keys.sorted { String(describing: $0) < String(describing: $1) }
    for key in keys {
        let desc = String(describing: info[key])
        print("KEY: \(key)")
        print("  VALUE: \(desc.count > 300 ? desc.prefix(300) + "…" : desc)")
    }
    getIsPlaying(queue) { playing in
        print("IS PLAYING: \(playing)")
        sem.signal()
    }
}

if sem.wait(timeout: .now() + 3) == .timedOut {
    print("TIMEOUT: callback never fired")
    exit(1)
}
