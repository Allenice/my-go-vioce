// macOS SVG rasterization for the Go Voice asset generator.
// Bun.Image handles raster resizing but cannot decode SVG.
import AppKit

if CommandLine.arguments.count != 3 {
    fatalError("Usage: swift render-go-voice-logo.swift source.svg output.png")
}

guard let image = NSImage(contentsOfFile: CommandLine.arguments[1]),
      let tiff = image.tiffRepresentation,
      let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else {
    fatalError("Cannot rasterize Go Voice SVG")
}
try png.write(to: URL(fileURLWithPath: CommandLine.arguments[2]), options: .atomic)
