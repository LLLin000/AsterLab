"""Generate 162 deterministic microscopy fixtures for QA-HUVEC-01.

9 wells x 3 timepoints x 6 FOV = 162 files, alternating PNG/TIFF (FOV odd =
PNG, even = TIFF), 64x48 8-bit grayscale with a per-coordinate pattern so
every image is visually distinct on the Notebook board and in the PDF.

Usage: python scripts/qa-huvec-images.py [outdir]
Default outdir: D:/.ci-cache/qa-huvec-01-assets
"""
import os, struct, sys, zlib

WELLS = ["A1", "A2", "A3", "B1", "B2", "B3", "C1", "C2", "C3"]
TPS = ["6h", "12h", "24h"]
W, H = 64, 48


def pixels(wi, ti, fi):
    base = (40 + wi * 17 + ti * 31 + fi * 47) % 180
    out = bytearray()
    for y in range(H):
        for x in range(W):
            v = base + ((x * 3 + y * 5 + fi * 37) % 60)
            if (x + y) % 16 == 0:
                v += 40  # diagonal texture ("tubes")
            if x < 2 or y < 2 or x >= W - 2 or y >= H - 2:
                v = 20 + fi * 10  # border encodes FOV
            out.append(v % 256)
    return bytes(out)


def write_png(path, raw):
    def chunk(typ, data):
        c = struct.pack(">I", len(data)) + typ + data
        return c + struct.pack(">I", zlib.crc32(typ + data) & 0xFFFFFFFF)

    ihdr = struct.pack(">IIBBBBB", W, H, 8, 0, 0, 0, 0)
    stride = W
    scan = b"".join(b"\x00" + raw[y * stride:(y + 1) * stride] for y in range(H))
    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n")
        f.write(chunk(b"IHDR", ihdr))
        f.write(chunk(b"IDAT", zlib.compress(scan, 9)))
        f.write(chunk(b"IEND", b""))


def write_tiff(path, raw):
    n = len(raw)
    header = struct.pack("<2sHI", b"II", 42, 8)
    count = 9
    ifd = struct.pack("<H", count)
    data_off = 8 + 2 + count * 12 + 4
    entries = [
        (256, 4, 1, W),          # ImageWidth LONG
        (257, 4, 1, H),          # ImageLength LONG
        (258, 3, 1, 8),          # BitsPerSample SHORT
        (259, 3, 1, 1),          # Compression none
        (262, 3, 1, 1),          # Photometric BlackIsZero
        (273, 4, 1, data_off),   # StripOffsets
        (277, 3, 1, 1),          # SamplesPerPixel
        (278, 4, 1, H),          # RowsPerStrip
        (279, 4, 1, n),          # StripByteCounts
    ]
    for tag, typ, cnt, val in entries:
        if typ == 3:
            ifd += struct.pack("<HHI4s", tag, typ, cnt, struct.pack("<H", val) + b"\x00\x00")
        else:
            ifd += struct.pack("<HHII", tag, typ, cnt, val)
    ifd += struct.pack("<I", 0)
    with open(path, "wb") as f:
        f.write(header + ifd + raw)


def main():
    outdir = sys.argv[1] if len(sys.argv) > 1 else "D:/.ci-cache/qa-huvec-01-assets"
    names = []
    for wi, well in enumerate(WELLS):
        for ti, tp in enumerate(TPS):
            for fov in range(1, 7):
                ext = "png" if fov % 2 == 1 else "tif"
                rel = f"huvec/{well}/{tp}/fov{fov}.{ext}"
                names.append((wi, ti, well, tp, fov, rel))
                os.makedirs(os.path.dirname(os.path.join(outdir, *rel.split("/"))), exist_ok=True)
    for wi, ti, well, tp, fov, rel in names:
        raw = pixels(wi, ti, fov)
        p = os.path.join(outdir, *rel.split("/"))
        if rel.endswith(".png"):
            write_png(p, raw)
        else:
            write_tiff(p, raw)
    print(f"wrote {len(names)} fixtures under {outdir}")


if __name__ == "__main__":
    main()
