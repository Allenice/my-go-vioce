import XCTest

@testable import BridgeCore

final class ContractTests: XCTestCase {
  func testPCMCanonicalAndBounds() throws {
    let value = Data(repeating: 0, count: 640).base64EncodedString()
    let object: [String: Any] = [
      "type": "pcm", "sessionId": 1, "sequence": 0, "payloadBase64": value,
    ]
    let command = try Command(data: JSONSerialization.data(withJSONObject: object))
    XCTAssertEqual(try command.pcm().count, 640)
    let bad = try Command(
      data: Data(
        "{\"type\":\"pcm\",\"sessionId\":1,\"sequence\":15000,\"payloadBase64\":\"AA==\"}".utf8))
    XCTAssertThrowsError(try bad.pcm())
    XCTAssertThrowsError(try Command(data: Data(repeating: 32, count: 16385)))
  }
  func testPermissionCommandsAndStates() throws {
    let request = try Command(data: Data(#"{"type":"request-permission","text":"speech"}"#.utf8))
    XCTAssertEqual(request.text, "speech")
    XCTAssertThrowsError(
      try Command(data: Data(#"{"type":"request-permission","text":"camera"}"#.utf8)))
    XCTAssertNoThrow(try Command(data: Data(#"{"type":"permissions"}"#.utf8)))
    XCTAssertEqual(Permission.automationState(0), "granted")
    XCTAssertEqual(Permission.automationState(-1743), "denied")
    XCTAssertEqual(Permission.automationState(-1744), "undetermined")
    XCTAssertEqual(Permission.automationState(-600), "unknown")
    XCTAssertEqual(Permission.event("bluetooth", "granted")["permission"] as? String, "bluetooth")
  }
  func testSessionIdentity() {
    var guardState = SessionGuard()
    XCTAssertTrue(guardState.start(1))
    XCTAssertFalse(guardState.start(1))
    guardState.cancel(2)
    XCTAssertTrue(guardState.matches(1))
    guardState.cancel(1)
    XCTAssertTrue(guardState.start(2))
    XCTAssertFalse(guardState.matches(1))
  }
  func testExplicitDeviceSelectionAndMTU() {
    let id = UUID()
    XCTAssertFalse(VoiceContract.mayConnect(selected: nil, candidate: id))
    XCTAssertFalse(VoiceContract.mayConnect(selected: UUID(), candidate: id))
    XCTAssertTrue(VoiceContract.mayConnect(selected: id, candidate: id))
    XCTAssertFalse(VoiceContract.supportsAudio(writeWithoutResponse: 181))
    XCTAssertTrue(VoiceContract.supportsAudio(writeWithoutResponse: 182))
    XCTAssertEqual(VoiceContract.service, "676f0001-766f-6963-652d-617564696f00")
  }
  func testClipboardOwnership() {
    XCTAssertTrue(shouldRestoreClipboard(expected: 2, current: 2, marker: "a", session: "a"))
    XCTAssertFalse(shouldRestoreClipboard(expected: 2, current: 3, marker: "a", session: "a"))
    XCTAssertFalse(shouldRestoreClipboard(expected: 2, current: 2, marker: "b", session: "a"))
  }
}
