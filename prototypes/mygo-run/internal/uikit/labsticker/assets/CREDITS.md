# Labware Sticker Assets Provenance & License Notice

The vector laboratory vessel assets in this directory (`02_ep_1_5ml.svg` through `12_erlenmeyer_250ml.svg`) are adapted from the **NIAID NIH BIOART Source** (<https://bioart.niaid.nih.gov/>), which releases scientific illustrations into the **Public Domain** (credit: *Courtesy of NIAID, NIH BioArt Source*):

- `02_ep_1_5ml.svg`: adapted from `BIOART-000143` (*Eppendorf Tube*, Ryan Kissinger, Courtesy of NIAID)
- `03_cryovial_2ml.svg`: adapted from `BIOART-000713` (*Cryotube*, Courtesy of NIAID)
- `04_screw_tube_2ml.svg`: adapted from `BIOART-000971` (*Screw Cap Tube*, Courtesy of NIAID)
- `05_amber_microvial.svg`: inspired by `BIOART-000964` (*Vial with Liquid*, Courtesy of NIAID)
- `06_serum_vial.svg`: adapted from `BIOART-000966` (*Prescription Vaccine Vial*, Courtesy of NIAID)
- `07_falcon_15ml.svg`, `08_falcon_50ml.svg`: adapted from `BIOART-000708` (*Conical Sample Tube*, Courtesy of NIAID)
- `09_media_bottle_500ml.svg`, `10_reagent_bottle_gl45.svg`: adapted from `BIOART-000801` (*Bottle*, Courtesy of NIAID)
- `11_amber_bottle_500ml.svg`: adapted from `BIOART-000922` (*Paraformaldehyde*, Courtesy of NIAID; stripped drug text & hardened outlines)
- `12_erlenmeyer_250ml.svg`: adapted from `BIOART-000631` (*Erlenmeyer Flask Filled*, Courtesy of NIAID)
- `01_pcr_strip.svg`: original AsterLab artwork styled in cohesive match with the above vessels.

Modifications made for AsterLab UI:
- Normalized XML namespaces, removed unsupported CSS `mix-blend-mode` rules, stripped hardcoded sample text (`RX`, `Paraformaldehyde`), and harmonized BioRender-style soft slate outlines, translucent glass/liquid gradients, meniscus highlights, and graduation ticks.
- Composited at runtime with an opaque white 2D Euclidean Distance Transform (SDF) die-cut vinyl border via `internal/uikit/labsticker`.
