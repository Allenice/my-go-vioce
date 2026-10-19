import Foundation
// swift-tools-version: 6.0
import PackageDescription

let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().path
func info(_ target: String) -> [LinkerSetting] {
  [
    .unsafeFlags([
      "-Xlinker", "-sectcreate", "-Xlinker", "__TEXT", "-Xlinker", "__info_plist", "-Xlinker",
      root + "/" + target + "/Info.plist",
    ])
  ]
}
let package = Package(
  name: "GoVoiceNative", platforms: [.macOS("26.0")],
  products: [
    .executable(name: "BluetoothHelper", targets: ["BluetoothHelper"]),
    .executable(name: "SpeechBridge", targets: ["SpeechBridge"]),
  ],
  targets: [
    .executableTarget(
      name: "BluetoothHelper", dependencies: ["BridgeCore"], path: "BluetoothHelper",
      exclude: ["Info.plist"], linkerSettings: info("BluetoothHelper")),
    .target(name: "BridgeCore", path: "BridgeCore"),
    .executableTarget(
      name: "SpeechBridge", dependencies: ["BridgeCore"], path: "SpeechBridge",
      exclude: ["Info.plist"], linkerSettings: info("SpeechBridge")),
    .testTarget(name: "BridgeCoreTests", dependencies: ["BridgeCore"], path: "Tests"),
  ], swiftLanguageModes: [.v5])
