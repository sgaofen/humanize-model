"""humanizer: rewrite AI-written drafts so they read like a person wrote them (a 12B text-completion model).

The installed command is `hz` (humanizer/hz.py): `hz draft.txt`, `hz paper.md -o out.md`, `hz --help`.
The other files in this folder are standalone scripts (inference helpers, metrics, deployment) and are
not imported by this package; run them as before, e.g. `python humanizer/humanize.py ...`.
"""
__version__ = "0.1.0"
