#!/usr/bin/env python3
"""Generate an 8-bit grayscale PNG calibration image using only stdlib."""
import struct
import zlib
from pathlib import Path

W, H = 600, 800
pixels = bytearray([255]) * (W * H)

def rect(x, y, w, h, shade):
    for row in range(y, y + h):
        pixels[row * W + x:row * W + x + w] = bytes([shade]) * w

# Inset border, four dashboard panels, grayscale ramp.
for x, y, w, h in [(8, 8, 584, 3), (8, 789, 584, 3),
                    (8, 8, 3, 784), (589, 8, 3, 784)]:
    rect(x, y, w, h, 0)
for x in (24, 312):
    for y in (140, 430):
        rect(x, y, 264, 260, 0)
        rect(x + 3, y + 3, 258, 254, 255)
        rect(x + 12, y + 16, 240, 8, 0)
        for i in range(4):
            rect(x + 16, y + 48 + i * 45, 160 - i * 20, 5, 0)
for i in range(16):
    rect(28 + i * 34, 725, 34, 35, i * 17)

def chunk(kind, data):
    return (struct.pack('>I', len(data)) + kind + data
            + struct.pack('>I', zlib.crc32(kind + data) & 0xffffffff))

raw = b''.join(b'\0' + pixels[y * W:(y + 1) * W] for y in range(H))
png = (b'\x89PNG\r\n\x1a\n'
       + chunk(b'IHDR', struct.pack('>IIBBBBB', W, H, 8, 0, 0, 0, 0))
       + chunk(b'IDAT', zlib.compress(raw)) + chunk(b'IEND', b''))
Path(__file__).with_name('screen-test.png').write_bytes(png)
