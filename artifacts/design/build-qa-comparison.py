from pathlib import Path

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parent
source = Image.open(ROOT / "auraedu-unified-target.png").convert("RGB")
implementation = Image.open(ROOT / "audit-current" / "home-desktop.png").convert("RGB")

panel_size = (768, 512)
source_panel = source.resize(panel_size, Image.Resampling.LANCZOS)

hero_crop = implementation.crop((0, 0, implementation.width, min(960, implementation.height)))
hero_panel = hero_crop.resize(panel_size, Image.Resampling.LANCZOS)

canvas = Image.new("RGB", (panel_size[0] * 2, panel_size[1] + 42), "white")
canvas.paste(source_panel, (0, 42))
canvas.paste(hero_panel, (panel_size[0], 42))

draw = ImageDraw.Draw(canvas)
draw.text((18, 14), "SOURCE TARGET", fill="#061631")
draw.text((panel_size[0] + 18, 14), "IMPLEMENTATION - DESKTOP HERO", fill="#061631")
draw.line((panel_size[0], 0, panel_size[0], canvas.height), fill="#dbe3ed", width=2)

canvas.save(ROOT / "design-qa-comparison-hero.png", optimize=True)
