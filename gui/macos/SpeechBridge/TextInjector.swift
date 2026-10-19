import AppKit
import ApplicationServices
import BridgeCore
import Carbon

@MainActor final class TextInjector {
  private var target: NSRunningApplication?
  private let marker = NSPasteboard.PasteboardType("yz.sh.mygo-go-voice.session")
  func capture() { target = NSWorkspace.shared.frontmostApplication }
  func key(_ code: CGKeyCode, flags: CGEventFlags = []) {
    guard !IsSecureEventInputEnabled(), AXIsProcessTrusted() else { return }
    let source = CGEventSource(stateID: .hidSystemState)
    let down = CGEvent(keyboardEventSource: source, virtualKey: code, keyDown: true)
    let up = CGEvent(keyboardEventSource: source, virtualKey: code, keyDown: false)
    down?.flags = flags
    up?.flags = flags
    down?.post(tap: .cghidEventTap)
    up?.post(tap: .cghidEventTap)
  }
  func deliver(_ text: String, session: UInt32, valid: () -> Bool) async throws -> String {
    guard valid() else { throw BridgeError.stale }
    let board = NSPasteboard.general
    let old = (board.pasteboardItems ?? []).map { item in
      Dictionary(
        uniqueKeysWithValues: item.types.compactMap { type in
          item.data(forType: type).map { (type, $0) }
        })
    }
    let token = UUID().uuidString
    board.clearContents()
    board.setString(text, forType: .string)
    board.setString(token, forType: marker)
    board.setString("", forType: NSPasteboard.PasteboardType("org.nspasteboard.TransientType"))
    let count = board.changeCount
    guard !IsSecureEventInputEnabled(), AXIsProcessTrusted() else { return "clipboard only" }
    guard let target, !target.isTerminated else { return "clipboard only; target unavailable" }
    if NSWorkspace.shared.frontmostApplication?.processIdentifier != target.processIdentifier {
      guard target.activate(options: []) else { return "clipboard only; target unavailable" }
    }
    let mask: CGEventFlags = [
      .maskCommand, .maskShift, .maskAlternate, .maskControl, .maskSecondaryFn,
    ]
    for _ in 0..<100 {
      guard valid() else { throw BridgeError.stale }
      if CGEventSource.flagsState(.combinedSessionState).intersection(mask).isEmpty { break }
      try await Task.sleep(for: .milliseconds(20))
    }
    guard CGEventSource.flagsState(.combinedSessionState).intersection(mask).isEmpty else {
      return "clipboard only; modifiers held"
    }
    try await Task.sleep(for: .milliseconds(30))
    guard valid(), !IsSecureEventInputEnabled() else { throw BridgeError.stale }
    guard NSWorkspace.shared.frontmostApplication?.processIdentifier == target.processIdentifier
    else { return "clipboard only; target changed" }
    var error: NSDictionary?
    NSAppleScript(
      source: "tell application \"System Events\" to keystroke \"v\" using command down")?
      .executeAndReturnError(&error)
    if error != nil {
      let source = CGEventSource(stateID: .hidSystemState)
      let flags = CGEventFlags(rawValue: CGEventFlags.maskCommand.rawValue | 0x8)
      let commandDown = CGEvent(keyboardEventSource: source, virtualKey: 55, keyDown: true)
      commandDown?.flags = flags
      commandDown?.post(tap: .cghidEventTap)
      key(9, flags: flags)
      CGEvent(keyboardEventSource: source, virtualKey: 55, keyDown: false)?.post(
        tap: .cghidEventTap)
    }
    Task { @MainActor in
      try? await Task.sleep(for: .milliseconds(500))
      guard
        shouldRestoreClipboard(
          expected: count, current: board.changeCount, marker: board.string(forType: self.marker),
          session: token)
      else { return }
      board.clearContents()
      let items = old.map { values -> NSPasteboardItem in
        let item = NSPasteboardItem()
        for (type, data) in values { item.setData(data, forType: type) }
        return item
      }
      board.writeObjects(items)
    }
    return "inserted"
  }
}

private let systemEventsID = "com.apple.systemevents"

/// Whether go-voice may send Apple Events to System Events. Asking launches
/// System Events in the background first, since macOS can only answer for a
/// running target, then shows the consent prompt if still undecided.
func automationState(ask: Bool) async -> String {
  if NSRunningApplication.runningApplications(withBundleIdentifier: systemEventsID).isEmpty {
    guard ask else { return "unknown" }
    let configuration = NSWorkspace.OpenConfiguration()
    configuration.activates = false
    configuration.addsToRecentItems = false
    _ = try? await NSWorkspace.shared.openApplication(
      at: URL(fileURLWithPath: "/System/Library/CoreServices/System Events.app"),
      configuration: configuration)
    for _ in 0..<20 {
      if !NSRunningApplication.runningApplications(withBundleIdentifier: systemEventsID).isEmpty {
        break
      }
      try? await Task.sleep(for: .milliseconds(100))
    }
  }
  // The consent prompt blocks its caller: keep it off the main thread.
  let status = await Task.detached { () -> OSStatus in
    var target = AEAddressDesc()
    let id = Array(systemEventsID.utf8)
    guard AECreateDesc(DescType(typeApplicationBundleID), id, id.count, &target) == noErr else {
      return OSStatus(procNotFound)
    }
    defer { AEDisposeDesc(&target) }
    return AEDeterminePermissionToAutomateTarget(
      &target, AEEventClass(typeWildCard), AEEventID(typeWildCard), ask)
  }.value
  return Permission.automationState(status)
}
