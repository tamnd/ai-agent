# Third-party parts

The binary and the site use these parts from other projects. Each part keeps its own licence.

| Part | Version | Use | Licence |
| --- | --- | --- | --- |
| [Inter](https://rsms.me/inter/) | 4 | The font of the site, in `site/assets/fonts` | SIL Open Font License 1.1 |
| [KaTeX](https://katex.org) | 0.19.1 | Renders math at build time, in `katex` | MIT |
| [goldmark](https://github.com/yuin/goldmark) | 1.8.7 | Markdown | MIT |
| [Chroma](https://github.com/alecthomas/chroma) | 2.27.0 | Code highlighting | MIT |
| [esbuild](https://github.com/evanw/esbuild) | 0.28.2 | Bundles the scenes | MIT |
| [goja](https://github.com/dop251/goja) | see `go.mod` | Runs KaTeX and the scene checks | MIT |
| [go-yaml](https://github.com/yaml/go-yaml) | 3.0.6 | YAML | MIT and Apache 2.0 |

The fonts of KaTeX are in `katex/assets/fonts` under the SIL Open Font License 1.1. The licence texts are in `LICENSES/`, `katex/assets/LICENSE` and `site/assets/fonts/LICENSE-Inter.txt`.
