# Changelog

## [1.5.0](https://github.com/bluefunda/cai-cli/compare/v1.4.0...v1.5.0) (2026-05-12)


### Features

* add ai code command with local filesystem agentic loop ([#36](https://github.com/bluefunda/cai-cli/issues/36)) ([7356abb](https://github.com/bluefunda/cai-cli/commit/7356abb6b439b7876cdbf0b62629c7237aef59f6))

## [1.4.0](https://github.com/bluefunda/cai-cli/compare/v1.3.3...v1.4.0) (2026-04-24)


### Features

* add Docker image publishing to Docker Hub and ghcr.io ([#33](https://github.com/bluefunda/cai-cli/issues/33)) ([4603477](https://github.com/bluefunda/cai-cli/commit/4603477ba286866773266079054e3dd471650e01))


### Bug Fixes

* use DOCKER_USERNAME/DOCKER_PASSWORD org secrets for Docker Hub ([#35](https://github.com/bluefunda/cai-cli/issues/35)) ([ca34a55](https://github.com/bluefunda/cai-cli/commit/ca34a5533af92a58c6ee94ba4dea0bfd1eca82c7))

## [1.3.3](https://github.com/bluefunda/cai-cli/compare/v1.3.2...v1.3.3) (2026-04-23)


### Bug Fixes

* align HOMEBREW_TAP_TOKEN env var name with reusable workflow ([#31](https://github.com/bluefunda/cai-cli/issues/31)) ([138fb4d](https://github.com/bluefunda/cai-cli/commit/138fb4dd540bf471b0f8f75398256ea656346943))

## [1.3.2](https://github.com/bluefunda/cai-cli/compare/v1.3.1...v1.3.2) (2026-04-23)


### Bug Fixes

* use GITHUB_TOKEN instead of GH_PAT in homebrew-patch job ([#29](https://github.com/bluefunda/cai-cli/issues/29)) ([a2839e5](https://github.com/bluefunda/cai-cli/commit/a2839e59a457f48b09166fced14d2306a70c8eca))

## [1.3.1](https://github.com/bluefunda/cai-cli/compare/v1.3.0...v1.3.1) (2026-04-23)


### Bug Fixes

* remove deprecated brews key, use homebrew_casks only ([#27](https://github.com/bluefunda/cai-cli/issues/27)) ([a0604b7](https://github.com/bluefunda/cai-cli/commit/a0604b787952b90bfedcbbc5bd8c14dc086f3cfa))

## [1.3.0](https://github.com/bluefunda/cai-cli/compare/v1.2.3...v1.3.0) (2026-04-21)


### Features

* add Homebrew Formula for Linux and macOS install ([#23](https://github.com/bluefunda/cai-cli/issues/23)) ([1e47a18](https://github.com/bluefunda/cai-cli/commit/1e47a18a68485ea53ce6033bbad957a358291f9d))
* add install.sh for one-line installation ([#26](https://github.com/bluefunda/cai-cli/issues/26)) ([22d6fb1](https://github.com/bluefunda/cai-cli/commit/22d6fb13cf03f95014a849bef0ba5873363fe3c0))

## [1.2.3](https://github.com/bluefunda/cai-cli/compare/v1.2.2...v1.2.3) (2026-03-10)


### Bug Fixes

* handle unclosed &lt;think&gt; tags in streaming filter (Sarvam) ([#18](https://github.com/bluefunda/cai-cli/issues/18)) ([fd2043d](https://github.com/bluefunda/cai-cli/commit/fd2043ddf6912bd7a26221d0bc613443816db8fc))

## [1.2.2](https://github.com/bluefunda/cai-cli/compare/v1.2.1...v1.2.2) (2026-03-10)


### Bug Fixes

* pass user prompt to GenerateTitle for chat title generation ([#17](https://github.com/bluefunda/cai-cli/issues/17)) ([202a9cd](https://github.com/bluefunda/cai-cli/commit/202a9cd795a02f144afb9166201c0232b2ee097d))
* strip &lt;think&gt; tags from LLM streaming output ([#14](https://github.com/bluefunda/cai-cli/issues/14)) ([fc2a496](https://github.com/bluefunda/cai-cli/commit/fc2a4960aea01abd961a5e44fb34abea735daf0c))

## [1.2.1](https://github.com/bluefunda/cai-cli/compare/v1.2.0...v1.2.1) (2026-02-18)


### Bug Fixes

* homebrew-patch token and standardize release workflow ([#10](https://github.com/bluefunda/cai-cli/issues/10)) ([bc661bf](https://github.com/bluefunda/cai-cli/commit/bc661bfb3e96bccbf00e614541992b2fda0f1265))

## [1.2.0](https://github.com/bluefunda/cai-cli/compare/v1.1.1...v1.2.0) (2026-02-18)


### Features

* **auth:** resolve realm from JWT and add --realm login flag ([#9](https://github.com/bluefunda/cai-cli/issues/9)) ([a38c11c](https://github.com/bluefunda/cai-cli/commit/a38c11c4297cfaeaacfc4224de3c828cff546678))
* graceful session recovery in chat REPL ([06ab454](https://github.com/bluefunda/cai-cli/commit/06ab454dcbfa44df3e2618defb9fb5a5b1c3f290))


### Bug Fixes

* patch homebrew cask with API asset URLs after release ([dfedeed](https://github.com/bluefunda/cai-cli/commit/dfedeede4f01a2d70a1b412b9e02c3e50fa23fc8))

## [1.1.1](https://github.com/bluefunda/cai-cli/compare/v1.1.0...v1.1.1) (2026-02-09)


### Bug Fixes

* auto-generate chat title after first message ([0090e61](https://github.com/bluefunda/cai-cli/commit/0090e61b08c0191425ed67fd51c7672478155a25))

## [1.1.0](https://github.com/bluefunda/cai-cli/compare/v1.0.0...v1.1.0) (2026-02-09)


### Features

* add .deb/.rpm packages and Homebrew cask ([c58ebb2](https://github.com/bluefunda/cai-cli/commit/c58ebb226a51b5b1510fb38e71ec5ff09f531d66))
* add .deb/.rpm packages and Homebrew cask to GoReleaser ([cc0bf8b](https://github.com/bluefunda/cai-cli/commit/cc0bf8b5cd950786693954515e82cb38b5aec1b3))


### Bug Fixes

* combine Release Please and GoReleaser into single workflow ([#4](https://github.com/bluefunda/cai-cli/issues/4)) ([94b2b14](https://github.com/bluefunda/cai-cli/commit/94b2b14746e0a653064dbe14b2bafa353ff519ef))

## 1.0.0 (2026-02-03)


### Features

* add Release Please for automated versioning ([#2](https://github.com/bluefunda/cai-cli/issues/2)) ([e870d98](https://github.com/bluefunda/cai-cli/commit/e870d985f927610dffb20dbf0e35139a520b97cc))
