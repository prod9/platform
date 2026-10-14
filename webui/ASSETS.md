# v2027 visual assets

Platform self-hosts the selected PRODIGY9 v2027 assets; builds and runtime do not
require the marketing-site checkout or a font CDN.

Source repository: `prod9/prodigy9.co`, revision
`04ce0e59e55a8fc7963e50cbc542994aefbf7b90`.

- `static/logo/wordmark-red.png` copies
  `logo_bundle/akira/transparent/wordmark-red.png`, an approved monocolor master.
- `static/art/akira-cap-full-1.webp` copies the matching `public/art/` mask;
  the packed white image carries the art in its alpha channel, consumed by the
  default CSS alpha mask at 6% strength with 1.25 cap gain. Its representation
  follows upstream `scripts/pack-mask.sh`; no luminance-mask override is needed.
- `static/fonts/` copies the referenced `public/fonts/` subsets: Iosevka Aile
  400/700, Iosevka 400/700, Iosevka Curly 800 extension, Mona Sans Extra Expanded
  700 extension, and IBM Plex Sans Thai 400/700.
- `src/lib/styles/fonts.css` adapts the generated `src/styles/fonts.css` font
  declarations and metric-matched fallbacks. Curly 800 and Mona Sans 700 core
  subsets remain embedded in that CSS; extension subsets load by Unicode range.
  Body and data faces retain optional loading; Thai subsets load only for Thai.
- `src/lib/styles/diagonals.css` and `depth.css` adapt the corresponding public
  v2027 styles for the controls used by platform. Chamfer geometry is shared
  by the painted faces; native controls remain unclipped for keyboard focus.

Font generation is an upstream, out-of-band operation. Platform consumes these
finished assets; no upstream font-generation script runs during its UI build.
The visual contract and behavior-preservation boundary live in
[`docs/spec/webui.md`](../docs/spec/webui.md#v2027-visual-system).
