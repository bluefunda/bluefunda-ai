# Changelog

## [1.20.2](https://github.com/bluefunda/bluefunda-ai/compare/v1.20.1...v1.20.2) (2026-06-22)


### Bug Fixes

* **update:** bump go-update to v0.1.1, remove workaround ([#121](https://github.com/bluefunda/bluefunda-ai/issues/121)) ([f9286de](https://github.com/bluefunda/bluefunda-ai/commit/f9286de13762b2fa0073d4c3b18b9932e5e01ca2))

## [1.20.1](https://github.com/bluefunda/bluefunda-ai/compare/v1.20.0...v1.20.1) (2026-06-22)


### Bug Fixes

* **update:** trust Homebrew tap + feat(docs): hero onboarding and env var fixes ([#119](https://github.com/bluefunda/bluefunda-ai/issues/119)) ([69058f9](https://github.com/bluefunda/bluefunda-ai/commit/69058f91a3fe79714ca1edad45dcb6f390fa0e2d))

## [1.20.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.19.1...v1.20.0) (2026-06-22)


### Features

* **crypto,sdk:** add token encryption at rest and Go SDK client ([#117](https://github.com/bluefunda/bluefunda-ai/issues/117)) ([6d5ffd3](https://github.com/bluefunda/bluefunda-ai/commit/6d5ffd339c01060fd61d6ce933b88165e6961f13))

## [1.19.1](https://github.com/bluefunda/bluefunda-ai/compare/v1.19.0...v1.19.1) (2026-06-22)


### Bug Fixes

* **tui:** Ctrl+C cancels current turn instead of quitting ([#115](https://github.com/bluefunda/bluefunda-ai/issues/115)) ([ed31958](https://github.com/bluefunda/bluefunda-ai/commit/ed31958db028d42b2319663f24be4321fce45393))

## [1.19.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.18.0...v1.19.0) (2026-06-22)


### Features

* **test:** add coverage increase plan under docs/ ([#112](https://github.com/bluefunda/bluefunda-ai/issues/112)) ([cec1f5e](https://github.com/bluefunda/bluefunda-ai/commit/cec1f5e561941960eca60368ebfa0bde4e6a35f7))

## [1.18.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.17.0...v1.18.0) (2026-06-22)


### Features

* **tui:** show token count in header ([#111](https://github.com/bluefunda/bluefunda-ai/issues/111)) ([dd48c06](https://github.com/bluefunda/bluefunda-ai/commit/dd48c068da723fa9dc24caf49a1ce6918b930f00))

## [1.17.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.16.0...v1.17.0) (2026-06-21)


### Features

* **cmd:** merge bai and bai code into a single bai command ([#108](https://github.com/bluefunda/bluefunda-ai/issues/108)) ([#109](https://github.com/bluefunda/bluefunda-ai/issues/109)) ([f3f3c9e](https://github.com/bluefunda/bluefunda-ai/commit/f3f3c9e8b4ba3a1f048f1ef45e87f264c7b23fe6))

## [1.16.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.15.0...v1.16.0) (2026-06-21)


### Features

* **cmd:** model aliases — auto/fast/think, default auto ([#104](https://github.com/bluefunda/bluefunda-ai/issues/104)) ([#105](https://github.com/bluefunda/bluefunda-ai/issues/105)) ([0aecc34](https://github.com/bluefunda/bluefunda-ai/commit/0aecc34bece1801fd7fce723b318cce62372d84c))

## [1.15.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.14.0...v1.15.0) (2026-06-21)


### Features

* **cmd:** context auto-compaction in bai code agentic loop ([#101](https://github.com/bluefunda/bluefunda-ai/issues/101)) ([b892d51](https://github.com/bluefunda/bluefunda-ai/commit/b892d51de7e59715056fa7850bcbeafcd7867cac))

## [1.14.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.13.1...v1.14.0) (2026-06-21)


### Features

* **tui:** track cumulative token usage from stream_end events ([#99](https://github.com/bluefunda/bluefunda-ai/issues/99)) ([6e5c442](https://github.com/bluefunda/bluefunda-ai/commit/6e5c4420f5c5cbce0d428f55ce3ff8ddd1a30dea))

## [1.13.1](https://github.com/bluefunda/bluefunda-ai/compare/v1.13.0...v1.13.1) (2026-06-21)


### Bug Fixes

* **cmd:** add version guard to cliCodePayload ([#84](https://github.com/bluefunda/bluefunda-ai/issues/84)) ([#97](https://github.com/bluefunda/bluefunda-ai/issues/97)) ([4f60a6f](https://github.com/bluefunda/bluefunda-ai/commit/4f60a6fd19af2195a97adeb43d001443fc222ea3))

## [1.13.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.12.1...v1.13.0) (2026-06-21)


### Features

* **ux:** --continue flag, shell completion, sessions view, rg check, /tools update ([#95](https://github.com/bluefunda/bluefunda-ai/issues/95)) ([6f53324](https://github.com/bluefunda/bluefunda-ai/commit/6f53324630d12c5347edaab3716d29fa98cfb80e))
* **ux:** --version flag, custom slash commands, parallel tools, WebFetch/WebSearch ([#93](https://github.com/bluefunda/bluefunda-ai/issues/93)) ([fd875ec](https://github.com/bluefunda/bluefunda-ai/commit/fd875ecf2f33c7000c851f6f31bcfde4f96572ba))


### Bug Fixes

* **lint:** QF1012 WriteString+Sprintf→Fprintf; QF1001 De Morgan's law ([#96](https://github.com/bluefunda/bluefunda-ai/issues/96)) ([20d8994](https://github.com/bluefunda/bluefunda-ai/commit/20d8994e4a5bcfff1cce343806ab2db8ad8ca283))

## [1.12.1](https://github.com/bluefunda/bluefunda-ai/compare/v1.12.0...v1.12.1) (2026-06-21)


### Bug Fixes

* **docker:** install git in builder stage; bump base image to golang:1.26 ([#91](https://github.com/bluefunda/bluefunda-ai/issues/91)) ([66ae177](https://github.com/bluefunda/bluefunda-ai/commit/66ae177a45a4307615e1eff96565837072f3622f))

## [1.12.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.11.0...v1.12.0) (2026-06-21)


### Features

* **code:** sprint 1 parity improvements — search, context, config, UX ([#86](https://github.com/bluefunda/bluefunda-ai/issues/86)) ([cd50cdb](https://github.com/bluefunda/bluefunda-ai/commit/cd50cdb2744f03da60cd13821ce7fd873965a223))
* **code:** sprint 2 — edit_file, sessions, hooks, audit, rate-limit backoff, headless mode ([#88](https://github.com/bluefunda/bluefunda-ai/issues/88)) ([157742a](https://github.com/bluefunda/bluefunda-ai/commit/157742ae50f732129d066be87dc9c43e9bf88311))
* **init,mcp:** bai init scaffold and local MCP client (stdio transport) ([#90](https://github.com/bluefunda/bluefunda-ai/issues/90)) ([d246715](https://github.com/bluefunda/bluefunda-ai/commit/d2467153469cf29af4ea918f922eda1d2f246859))


### Bug Fixes

* **lint:** add blank-identifier assignments for unchecked file-op error returns ([#89](https://github.com/bluefunda/bluefunda-ai/issues/89)) ([bbd9f5c](https://github.com/bluefunda/bluefunda-ai/commit/bbd9f5c4e9c600f1f005727543d66e94347a11a2))

## [1.11.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.10.0...v1.11.0) (2026-06-20)


### Features

* **update:** add `bai update` self-update command ([#65](https://github.com/bluefunda/bluefunda-ai/issues/65)) ([921ecda](https://github.com/bluefunda/bluefunda-ai/commit/921ecda4dbc06d1b63464440c2e7046fda8c05da))


### Bug Fixes

* **deps:** bump go to 1.25.11 to resolve GO-2026-5039 and GO-2026-5037 ([#67](https://github.com/bluefunda/bluefunda-ai/issues/67)) ([c8b699b](https://github.com/bluefunda/bluefunda-ai/commit/c8b699b00111921d2b037b45b8289e90476c90e0))

## [1.10.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.9.0...v1.10.0) (2026-05-23)


### Features

* bai v2.0.0 — AI-first CLI redesign ([#55](https://github.com/bluefunda/bluefunda-ai/issues/55)) ([33b2b4b](https://github.com/bluefunda/bluefunda-ai/commit/33b2b4b3468ca1b2eb6e10635c052bdf69331e5e))


### Bug Fixes

* resolve lint and govulncheck CI failures ([#58](https://github.com/bluefunda/bluefunda-ai/issues/58)) ([8112266](https://github.com/bluefunda/bluefunda-ai/commit/8112266932fdd2f5285168160ec848caffff7315))

## [1.9.0](https://github.com/bluefunda/bluefunda-ai/compare/v1.8.0...v1.9.0) (2026-05-23)


### Features

* rebrand cai-cli to BlueFunda AI (bai) ([#53](https://github.com/bluefunda/bluefunda-ai/issues/53)) ([604f24e](https://github.com/bluefunda/bluefunda-ai/commit/604f24ec308a971fbdb9b4666dbd9a3ac7c45679))

## [1.8.0](https://github.com/bluefunda/cai-cli/compare/v1.7.0...v1.8.0) (2026-05-22)


### Features

* replace REPL with full BubbleTea TUI ([#51](https://github.com/bluefunda/cai-cli/issues/51)) ([8478df6](https://github.com/bluefunda/cai-cli/commit/8478df6e03d41a865dbfae5bbe5752f52af743ee))

## [1.7.0](https://github.com/bluefunda/cai-cli/compare/v1.6.1...v1.7.0) (2026-05-20)


### Features

* enhance CLI stream rendering with tool visibility and spinner ([8f06524](https://github.com/bluefunda/cai-cli/commit/8f065245b3a5079360c154f355088b7f4d12deb3))

## [1.6.1](https://github.com/bluefunda/cai-cli/compare/v1.6.0...v1.6.1) (2026-05-13)


### Bug Fixes

* add workflow_dispatch and remove homebrew-patch job ([#43](https://github.com/bluefunda/cai-cli/issues/43)) ([024b4c1](https://github.com/bluefunda/cai-cli/commit/024b4c1a7d5a9edabcfddfb70dd0a312ca87e2ca))

## [1.6.0](https://github.com/bluefunda/cai-cli/compare/v1.5.1...v1.6.0) (2026-05-13)


### Features

* add macOS code signing and notarization ([#41](https://github.com/bluefunda/cai-cli/issues/41)) ([1a78714](https://github.com/bluefunda/cai-cli/commit/1a78714891731675174cf3af3d89420a14058b56))

## [1.5.1](https://github.com/bluefunda/cai-cli/compare/v1.5.0...v1.5.1) (2026-05-13)


### Bug Fixes

* encode local_tools and history in Prompt for ai code command ([#39](https://github.com/bluefunda/cai-cli/issues/39)) ([0cc1a00](https://github.com/bluefunda/cai-cli/commit/0cc1a0014298fe7470704aaa1c0ab6b3bd219d68))

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
