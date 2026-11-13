# Runner images

`aik test` runs the builds, the native tests, the adapters and the custom checkers in these images (spec 08.7). Each command gets a new container with no network, a read-only root, a memory limit of 2 GiB, a CPU limit of 2 and a process limit of 512. The build tree is the only writable mount.

| Image | Languages | Toolchain |
|---|---|---|
| `aik-runner-python` | python | Python 3.14, pytest 9.2 |
| `aik-runner-numpy` | numpy | Python 3.14, pytest 9.2, NumPy 2.5.4, SciPy 1.18.1, Hypothesis 6.170, ruff 0.17.5, basedpyright 1.40.2 |
| `aik-runner-pytorch` | pytorch | the NumPy set, and PyTorch 2.15.0 for CPU |
| `aik-runner-jax` | jax | the NumPy set, and JAX 0.12.0.1 with jaxlib for CPU |
| `aik-runner-go` | go | Go 1.27 |
| `aik-runner-rust` | rust | Rust 1.98 |
| `aik-runner-gcc` | c, cpp | GCC 15, with ASan and UBSan |
| `aik-runner-node` | javascript, typescript | Node 24 |
| `aik-runner-jvm` | java, kotlin | JDK 25, Kotlin 2.4.21, JUnit console launcher 1.13.4 |
| `aik-runner-dotnet` | csharp | .NET SDK 10, xUnit 2.9.3, restored ahead of time |
| `aik-runner-lean` | lean | Lean 4.35.0 |

The framework images carry ruff and basedpyright for the format, lint and type checks of X3 (spec 08.10); basedpyright brings its own Node as a wheel, so the checks need no network.

Each base image and each download is pinned by digest. The workflow `images.yml` builds the images, runs the sandbox tests on them, and pushes them to `ghcr.io/tamnd` with build provenance. The content repository pins each image by digest in `manifests/images.yaml`.

The images are for `linux/amd64`. On another machine, give `aik test -unsafe-local` to run the tests with the tools of the machine.

The images expect these paths:

- `/opt/algo/junit.jar` is the JUnit console launcher.
- `/opt/kotlinc` is the Kotlin compiler.
- `/opt/nuget` holds the NuGet packages of the test project. It is read-only, so a restore cannot fetch a new package.
