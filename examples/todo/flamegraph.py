#!/usr/bin/env python3
"""Render a pprof CPU profile as a self-contained flame graph SVG.

    go tool pprof -raw cpu.pb.gz > cpu.raw
    ./flamegraph.py cpu.raw out.svg "title"

pprof's own flame graph lives behind `-http`, which needs a browser. This
produces the same view as a file that can be committed or attached.
"""
import re
import sys
from html import escape


def parse(path):
    """Fold a `pprof -raw` dump into {stack tuple: sample count}."""
    text = open(path, encoding="utf-8", errors="replace").read()
    # Locations map an address to its function names, innermost first when a
    # frame was inlined.
    locations, in_locations = {}, False
    for line in text.splitlines():
        if line.startswith("Locations"):
            in_locations = True
            continue
        if line.startswith("Samples"):
            in_locations = False
            continue
        if in_locations:
            m = re.match(r"\s+(\d+):.*?\s+(\S+)\s+\S+:\d+", line)
            if m:
                locations.setdefault(m.group(1), []).append(m.group(2))
            else:
                m2 = re.match(r"\s+(\d+):", line)
                if m2:
                    locations.setdefault(m2.group(1), [])

    stacks, in_samples = {}, False
    for line in text.splitlines():
        if line.startswith("Samples"):
            in_samples = True
            continue
        if in_samples:
            m = re.match(r"\s+(\d[\d ]*):((?:\s+\d+)+)", line)
            if not m:
                continue
            counts = m.group(1).split()
            if len(counts) < 2:
                continue
            value = int(counts[1])  # second value is cpu nanoseconds
            addrs = m.group(2).split()
            frames = []
            for a in reversed(addrs):  # outermost first
                for fn in reversed(locations.get(a, [])):
                    frames.append(fn)
            if frames and value:
                stacks[tuple(frames)] = stacks.get(tuple(frames), 0) + value
    return stacks


def build(stacks):
    """Turn folded stacks into (depth, x, width, name) rectangles."""
    root = {"name": "all", "value": 0, "children": {}}
    for frames, value in stacks.items():
        node = root
        node["value"] += value
        for f in frames:
            node = node["children"].setdefault(f, {"name": f, "value": 0, "children": {}})
            node["value"] += value
    rects = []

    def walk(node, depth, x):
        rects.append((depth, x, node["value"], node["name"]))
        for child in sorted(node["children"].values(), key=lambda c: -c["value"]):
            walk(child, depth + 1, x)
            x += child["value"]

    walk(root, 0, 0)
    return rects, root["value"]


def color(name):
    h = 0
    for c in name:
        h = (h * 31 + ord(c)) & 0xFFFFFFFF
    return f"rgb({205 + h % 50},{h % 130},{h % 60})"


def render(rects, total, title, out):
    row, pad, width = 17, 3, 1400
    depth = max(r[0] for r in rects) + 1
    height = depth * row + 60
    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" '
        f'viewBox="0 0 {width} {height}" font-family="ui-monospace,monospace" font-size="11">',
        f'<rect width="{width}" height="{height}" fill="#f8f8f4"/>',
        f'<text x="{width // 2}" y="20" text-anchor="middle" font-size="14" font-weight="bold">{escape(title)}</text>',
    ]
    for d, x, v, name in rects:
        if total == 0:
            continue
        w = v * (width - 2 * pad) / total
        if w < 0.35:  # narrower than a pixel would be noise
            continue
        px = pad + x * (width - 2 * pad) / total
        py = height - (d + 1) * row
        pct = 100.0 * v / total
        label = ""
        if w > 42:
            budget = int(w // 6.2)
            short = name.rsplit("/", 1)[-1]
            label = short if len(short) <= budget else short[: max(0, budget - 1)] + "…"
        parts.append(
            f'<g><title>{escape(name)} — {pct:.2f}%</title>'
            f'<rect x="{px:.2f}" y="{py}" width="{w:.2f}" height="{row - 1}" fill="{color(name)}" '
            f'rx="1" stroke="#f8f8f4" stroke-width="0.5"/>'
            f'<text x="{px + 3:.2f}" y="{py + row - 5}" fill="#1a1a1a">{escape(label)}</text></g>'
        )
    parts.append("</svg>")
    open(out, "w", encoding="utf-8").write("\n".join(parts))


if __name__ == "__main__":
    raw, out = sys.argv[1], sys.argv[2]
    heading = sys.argv[3] if len(sys.argv) > 3 else "CPU"
    folded = parse(raw)
    rects, total = build(folded)
    render(rects, total, heading, out)
    print(f"{out}: {len(folded)} stacks, {total / 1e9:.2f}s CPU")
