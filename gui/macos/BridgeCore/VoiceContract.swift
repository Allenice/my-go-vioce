import Foundation

public enum VoiceContract {
  public static let service = "676f0001-766f-6963-652d-617564696f00"
  public static let control = "676f0002-766f-6963-652d-617564696f00"
  public static let audio = "676f0003-766f-6963-652d-617564696f00"
  public static let info = "676f0004-766f-6963-652d-617564696f00"
  public static func mayConnect(selected: UUID?, candidate: UUID) -> Bool { selected == candidate }
  public static func supportsAudio(writeWithoutResponse: Int) -> Bool {
    writeWithoutResponse >= 182
  }
}
