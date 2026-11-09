# Changelog

> **Note:** Prior to v2.0.0 this project was published as `github.com/bluefunda/abaper-cli`. Historical links in this changelog point to the old repository for reference.

## [1.17.0](https://github.com/bluefunda/abaper/compare/v1.16.10...v1.17.0) (2026-11-09)


### Features

* **status:** real health reporting — token validity, gateway, per-system SAP connectivity ([#142](https://github.com/bluefunda/abaper/issues/142)) ([859c0d6](https://github.com/bluefunda/abaper/commit/859c0d6f9ebce765dcafd3a44c8a7f8f343cb688))

## [1.16.10](https://github.com/bluefunda/abaper/compare/v1.16.9...v1.16.10) (2026-11-09)


### Bug Fixes

* **ai:** safe, self-verifying agent loop — no silent renames, abaper check, verification gate, TUI agent ([#160](https://github.com/bluefunda/abaper/issues/160)) ([b032ad1](https://github.com/bluefunda/abaper/commit/b032ad1d6f045cc4a7fdcfef77d476e527daf078))

## [1.16.9](https://github.com/bluefunda/abaper/compare/v1.16.8...v1.16.9) (2026-07-10)


### Bug Fixes

* **ai code:** encode live-verified ABAP/DDIC gotchas into the system prompt ([#132](https://github.com/bluefunda/abaper/issues/132)) ([e6319f8](https://github.com/bluefunda/abaper/commit/e6319f8992f0cc76e34bfc7144c94908b5482d7f))

## [1.16.8](https://github.com/bluefunda/abaper/compare/v1.16.7...v1.16.8) (2026-07-10)


### Bug Fixes

* **ai:** default model to fast; collapse tool-call noise behind --verbose ([#130](https://github.com/bluefunda/abaper/issues/130)) ([8325fac](https://github.com/bluefunda/abaper/commit/8325fac38200eb06a25c904efbbf18f364cddf27))

## [1.16.7](https://github.com/bluefunda/abaper/compare/v1.16.6...v1.16.7) (2026-07-09)


### Bug Fixes

* **client:** stop masking real ADT errors; test(adt,rest): add end-to-end coverage ([#128](https://github.com/bluefunda/abaper/issues/128)) ([7df11e1](https://github.com/bluefunda/abaper/commit/7df11e122d927d8174b7531627559700dc6d50b3))

## [1.16.6](https://github.com/bluefunda/abaper/compare/v1.16.5...v1.16.6) (2026-07-08)


### Bug Fixes

* **client:** send description on object create so server takes create path ([#121](https://github.com/bluefunda/abaper/issues/121)) ([cac7f8b](https://github.com/bluefunda/abaper/commit/cac7f8bb23afeb8bae8d4f401272ed57fdfd428f))

## [1.16.5](https://github.com/bluefunda/abaper/compare/v1.16.4...v1.16.5) (2026-07-08)


### Bug Fixes

* **login:** update login command description to reflect bai delegation ([#119](https://github.com/bluefunda/abaper/issues/119)) ([41c5ae5](https://github.com/bluefunda/abaper/commit/41c5ae5319317fdff83a53369b0c8238fe9ef785))

## [1.16.4](https://github.com/bluefunda/abaper/compare/v1.16.3...v1.16.4) (2026-07-08)


### Bug Fixes

* **ai:** retry with _V2/_V3 suffix when deploy returns already exists ([#115](https://github.com/bluefunda/abaper/issues/115)) ([4e230db](https://github.com/bluefunda/abaper/commit/4e230db8d6d50600d69189c8e071e04a30f819ba))

## [1.16.3](https://github.com/bluefunda/abaper/compare/v1.16.2...v1.16.3) (2026-07-08)


### Bug Fixes

* **rest:** auto-create program stub when deploy target doesn't exist ([#114](https://github.com/bluefunda/abaper/issues/114)) ([f005f80](https://github.com/bluefunda/abaper/commit/f005f805e28056ee1aeec3641fd76c6a7ea8ad23))

## [1.16.2](https://github.com/bluefunda/abaper/compare/v1.16.1...v1.16.2) (2026-07-08)


### Bug Fixes

* **ai:** suppress verbose prose in ai code system prompt ([#112](https://github.com/bluefunda/abaper/issues/112)) ([f748dd2](https://github.com/bluefunda/abaper/commit/f748dd26a3fe798242438c6bcb1be1d508d451e7))

## [1.16.1](https://github.com/bluefunda/abaper/compare/v1.16.0...v1.16.1) (2026-07-08)


### Bug Fixes

* **ai:** bump bluefunda-ai to v1.35.1 — fixes LLM routing failed on every prompt ([#110](https://github.com/bluefunda/abaper/issues/110)) ([d45995f](https://github.com/bluefunda/abaper/commit/d45995fe85a324af4a895b3c3ac00cacb0661c54))

## [1.16.0](https://github.com/bluefunda/abaper/compare/v1.15.1...v1.16.0) (2026-07-03)


### Features

* **adt:** OData V4 (RAP) service exposure — Service Definition + Service Binding ([#107](https://github.com/bluefunda/abaper/issues/107)) ([36821be](https://github.com/bluefunda/abaper/commit/36821be8cac296fe0f967d9462c080c78cad7013))

## [1.15.1](https://github.com/bluefunda/abaper/compare/v1.15.0...v1.15.1) (2026-07-02)


### Bug Fixes

* **adt:** use adtcore:-prefixed XML for DDIC property object activation ([#105](https://github.com/bluefunda/abaper/issues/105)) ([27b55bb](https://github.com/bluefunda/abaper/commit/27b55bbd6deeede07156283fc351b8f24f96bc7e))

## [1.15.0](https://github.com/bluefunda/abaper/compare/v1.14.0...v1.15.0) (2026-07-02)


### Features

* **adt:** domain/data-element create+update via structured-properties flow ([#102](https://github.com/bluefunda/abaper/issues/102)) ([b97e8dc](https://github.com/bluefunda/abaper/commit/b97e8dc6fcee4d06aad742a68b56acb29845ab90))

## [1.14.0](https://github.com/bluefunda/abaper/compare/v1.13.1...v1.14.0) (2026-07-02)


### Features

* **adt:** full CRUD parity for function modules, DDIC objects, CDS views ([#100](https://github.com/bluefunda/abaper/issues/100)) ([76a648d](https://github.com/bluefunda/abaper/commit/76a648d9410db67cb2b4ba5e999fc6c3fa7a1f3d))

## [1.13.1](https://github.com/bluefunda/abaper/compare/v1.13.0...v1.13.1) (2026-07-02)


### Bug Fixes

* **adt:** repair packages/contents parsing and enable DDIC creation ([#98](https://github.com/bluefunda/abaper/issues/98)) ([80c59c3](https://github.com/bluefunda/abaper/commit/80c59c383df2a728a4e656ce74b21d364fdf1284))

## [1.13.0](https://github.com/bluefunda/abaper/compare/v1.12.0...v1.13.0) (2026-07-02)


### Features

* **sdk:** add public SDK package for embedding the ADT client ([#96](https://github.com/bluefunda/abaper/issues/96)) ([618ba83](https://github.com/bluefunda/abaper/commit/618ba838d4aca47e149e9327189158e0624be3c9))

## [1.12.0](https://github.com/bluefunda/abaper/compare/v1.11.1...v1.12.0) (2026-07-02)


### Features

* **rest:** abaper-ts parity — activate alias, save-mode create, packages/contents ([#92](https://github.com/bluefunda/abaper/issues/92)) ([7baa0bd](https://github.com/bluefunda/abaper/commit/7baa0bdf92950ba8f52f0e5bee558da583193e48))

## [1.11.1](https://github.com/bluefunda/abaper/compare/v1.11.0...v1.11.1) (2026-07-02)


### Bug Fixes

* **list:** repair list objects/packages against REST server ([#87](https://github.com/bluefunda/abaper/issues/87)) ([3fe2af3](https://github.com/bluefunda/abaper/commit/3fe2af3aa890b5d4941ebb2a0cb51fbeb66b0534))

## [1.11.0](https://github.com/bluefunda/abaper/compare/v1.10.0...v1.11.0) (2026-06-24)


### Features

* **ai:** replace HTTP SSE gateway chat with bluefunda-ai embedded SDK ([#80](https://github.com/bluefunda/abaper/issues/80)) ([8317a78](https://github.com/bluefunda/abaper/commit/8317a78932f647762dfbeeebf2972ca1432a3553))

## [1.10.0](https://github.com/bluefunda/abaper/compare/v1.9.1...v1.10.0) (2026-06-20)


### Features

* **update:** add `abaper update` self-update command ([#75](https://github.com/bluefunda/abaper/issues/75)) ([3cefd68](https://github.com/bluefunda/abaper/commit/3cefd6866e5d2ae0b5577f167516b673370e286f))

## [1.9.1](https://github.com/bluefunda/abaper/compare/v1.9.0...v1.9.1) (2026-06-04)


### Bug Fixes

* **docker:** align builder image with go.mod (golang:1.26-alpine) ([#72](https://github.com/bluefunda/abaper/issues/72)) ([ff5f6f2](https://github.com/bluefunda/abaper/commit/ff5f6f2fced8beb0e94cc523d55b0a3c31644b9b))

## [1.9.0](https://github.com/bluefunda/abaper/compare/v1.8.0...v1.9.0) (2026-06-04)


### Features

* **serve:** allow starting without static SAP credentials ([#70](https://github.com/bluefunda/abaper/issues/70)) ([caa81bd](https://github.com/bluefunda/abaper/commit/caa81bd65583e54b636d78e2b7752e6edc8bc797))

## [1.8.0](https://github.com/bluefunda/abaper/compare/v1.7.1...v1.8.0) (2026-06-04)


### Features

* **adt:** add FormatSource (ABAP pretty-printer) to Go ADT SDK ([#59](https://github.com/bluefunda/abaper/issues/59)) ([22a0d37](https://github.com/bluefunda/abaper/commit/22a0d37213dc774f4eafd9c7e13d53270519509f))
* **adt:** add GetDDLSource (CDS/DDLS) to Go ADT SDK ([#65](https://github.com/bluefunda/abaper/issues/65)) ([8c2c1f7](https://github.com/bluefunda/abaper/commit/8c2c1f79312b660d0671973019ed34e1dc15be0d))
* **adt:** add GetTransportInfo and CreateTransport to Go ADT SDK ([#61](https://github.com/bluefunda/abaper/issues/61)) ([cdb114e](https://github.com/bluefunda/abaper/commit/cdb114ef410319f5bd70406e87f100e486112dd5))
* **adt:** add UpdateFunction and UpdateFunctionGroup to Go ADT SDK ([#63](https://github.com/bluefunda/abaper/issues/63)) ([182ac07](https://github.com/bluefunda/abaper/commit/182ac070cd9190fff45154024b18946333d9aea0))
* **rest:** abaper-ts parity — 8 new endpoints, SDK stubs, serve command ([#68](https://github.com/bluefunda/abaper/issues/68)) ([423a6be](https://github.com/bluefunda/abaper/commit/423a6be883dbb1d3828a29c254f2459f0b6b79fc))


### Bug Fixes

* **adt:** endpoint and parser fixes for 100% abaper-ts parity ([#69](https://github.com/bluefunda/abaper/issues/69)) ([c788dd8](https://github.com/bluefunda/abaper/commit/c788dd89f119aa701d343fdf308231d8da231032))
* **ci:** remove unsupported continue-on-error from release-notes job ([#53](https://github.com/bluefunda/abaper/issues/53)) ([3388ef2](https://github.com/bluefunda/abaper/commit/3388ef2bece2d1f015d6c325c0a675286e77b29f))
* **commands:** rename slash commands to avoid built-in skill conflicts ([#55](https://github.com/bluefunda/abaper/issues/55)) ([3ce30a1](https://github.com/bluefunda/abaper/commit/3ce30a15c675f296059574159faa4a1b8a710a29))
* **commands:** use 2&gt;&1 instead of 2>/dev/null in plan and work ([#56](https://github.com/bluefunda/abaper/issues/56)) ([bbdcd45](https://github.com/bluefunda/abaper/commit/bbdcd4547545e18f056b093032b16c1f0abe6457))

## [1.7.1](https://github.com/bluefunda/abaper/compare/v1.7.0...v1.7.1) (2026-05-29)


### Bug Fixes

* wait for Apple notarization before archiving macOS binaries ([#48](https://github.com/bluefunda/abaper/issues/48)) ([01df3da](https://github.com/bluefunda/abaper/commit/01df3daaa9540b140134578d153dd8e3b1574e43))

## [1.7.0](https://github.com/bluefunda/abaper/compare/v1.6.0...v1.7.0) (2026-05-29)


### Features

* **system:** add SAP system management to CLI and TUI ([3de5273](https://github.com/bluefunda/abaper/commit/3de52732ecde4cb13ff67ea3311ef789b14f9469))


### Bug Fixes

* **system:** inject X-SAP-* headers into StreamChat requests ([2a1c5b5](https://github.com/bluefunda/abaper/commit/2a1c5b5f68098cd13bbe527e2681c3105f206ec8))
* **system:** suppress unchecked errcheck on tabwriter fmt calls ([c55909d](https://github.com/bluefunda/abaper/commit/c55909d18e8e8e204233178c7f18479185a04a2e))

## [1.6.0](https://github.com/bluefunda/abaper/compare/v1.5.1...v1.6.0) (2026-05-29)


### Features

* Bubble Tea TUI, SDK reliability, and idiomatic Go refactor ([#39](https://github.com/bluefunda/abaper/issues/39)) ([05c12b8](https://github.com/bluefunda/abaper/commit/05c12b8dc2dd366140542a2cefed99dc4cbf0716))

## [1.5.1](https://github.com/bluefunda/abaper/compare/v1.5.0...v1.5.1) (2026-05-28)


### Bug Fixes

* add packages:write permission to docker-build workflow ([#34](https://github.com/bluefunda/abaper/issues/34)) ([b15562e](https://github.com/bluefunda/abaper/commit/b15562e37fdf02ae1ba3a8360c9ec0bf3ae737a1))
* remove private module setup from Dockerfile ([#37](https://github.com/bluefunda/abaper/issues/37)) ([2e91117](https://github.com/bluefunda/abaper/commit/2e91117af8b2b51db8d86313c0d1008ceda1a65d))

## [2.0.0](https://github.com/bluefunda/abaper/compare/v1.5.0...v2.0.0) (2026-05-28)

### Features

* merge CLI and Go SDK into unified `github.com/bluefunda/abaper` module
* add LSP server (`lsp/`), ADT client (`internal/adt/`), REST server (`rest/`), shared types (`types/`)
* standardize Apache 2.0 licensing

### ⚠ BREAKING CHANGES

* module path changed from `github.com/bluefunda/abaper-cli` to `github.com/bluefunda/abaper`

## [1.5.0](https://github.com/bluefunda/abaper-cli/compare/v1.4.4...v1.5.0) (2026-05-13)


### Features

* add macOS notarized brew cask installer ([abaa043](https://github.com/bluefunda/abaper-cli/commit/abaa04326c7d6e70ab08c5319e3427a160dceb04))

## [1.4.4](https://github.com/bluefunda/abaper-cli/compare/v1.4.3...v1.4.4) (2026-04-24)


### Bug Fixes

* use DOCKER_USERNAME/DOCKER_PASSWORD org secrets, add continue-on-error to description step ([c10ddcb](https://github.com/bluefunda/abaper-cli/commit/c10ddcbbe1e7a3be1ad9967867f8030105a49356))

## [1.4.3](https://github.com/bluefunda/abaper-cli/compare/v1.4.2...v1.4.3) (2026-04-24)


### Bug Fixes

* fold dockerhub-readme into docker job to fix 403 on description update ([e8f5513](https://github.com/bluefunda/abaper-cli/commit/e8f55137963f2990d08a5461325058604f3adb8e))

## [1.4.2](https://github.com/bluefunda/abaper-cli/compare/v1.4.1...v1.4.2) (2026-04-23)


### Bug Fixes

* replace fi with done in install.sh dependency check loop ([d2a3d2e](https://github.com/bluefunda/abaper-cli/commit/d2a3d2ef446646bbfd8ff8450efd0a72ca48f9df))

## [1.4.1](https://github.com/bluefunda/abaper-cli/compare/v1.4.0...v1.4.1) (2026-04-23)


### Bug Fixes

* correct docker image name from abaper to abaper-cli ([#22](https://github.com/bluefunda/abaper-cli/issues/22)) ([161c1e0](https://github.com/bluefunda/abaper-cli/commit/161c1e05e436b4524e89985e94a09becade4a242))

## [1.4.0](https://github.com/bluefunda/abaper-cli/compare/v1.3.1...v1.4.0) (2026-04-22)


### Features

* add install.sh for one-line installation ([#20](https://github.com/bluefunda/abaper-cli/issues/20)) ([dbd853e](https://github.com/bluefunda/abaper-cli/commit/dbd853ebc8d58865090a47a3363ff394e53ed5b9))

## [1.3.1](https://github.com/bluefunda/abaper-cli/compare/v1.3.0...v1.3.1) (2026-04-02)


### Bug Fixes

* revert auth host to auth.bluefunda.com ([#16](https://github.com/bluefunda/abaper-cli/issues/16)) ([b64205e](https://github.com/bluefunda/abaper-cli/commit/b64205e5b8965a1d527a330f2670ed460f835a76))

## [1.3.0](https://github.com/bluefunda/abaper-cli/compare/v1.2.1...v1.3.0) (2026-04-02)


### Features

* add telemetry, signup command, route login through bluefunda.com ([#14](https://github.com/bluefunda/abaper-cli/issues/14)) ([3fe4c64](https://github.com/bluefunda/abaper-cli/commit/3fe4c646c3ec4ecb2f396669a4801f29e8883274))

## [1.2.1](https://github.com/bluefunda/abaper-cli/compare/v1.2.0...v1.2.1) (2026-03-13)


### Bug Fixes

* switch from Homebrew cask to formula to avoid macOS Gatekeeper ([#11](https://github.com/bluefunda/abaper-cli/issues/11)) ([cd07ae7](https://github.com/bluefunda/abaper-cli/commit/cd07ae7a17ecf9e907ae40420e94f6654e1a0b94))

## [1.2.0](https://github.com/bluefunda/abaper-cli/compare/v1.1.1...v1.2.0) (2026-03-13)


### Features

* add AI chat, unit tests, list, and missing gateway API coverage ([#8](https://github.com/bluefunda/abaper-cli/issues/8)) ([96f6436](https://github.com/bluefunda/abaper-cli/commit/96f643628173bd9c83bff085c78c69b09390e190))


### Bug Fixes

* use RELEASE_PAT secret for release-please workflow ([#9](https://github.com/bluefunda/abaper-cli/issues/9)) ([62be342](https://github.com/bluefunda/abaper-cli/commit/62be34238745ecf1a00817d5cfc492062ba29a99))

## [1.1.1](https://github.com/bluefunda/abaper-cli/compare/v1.1.0...v1.1.1) (2026-03-11)


### Bug Fixes

* use homebrew_casks, HOMEBREW_TAP_TOKEN, and correct DOCKERHUB_TOKEN ([#6](https://github.com/bluefunda/abaper-cli/issues/6)) ([7dc3984](https://github.com/bluefunda/abaper-cli/commit/7dc39846a9555bbc02081b84c316fcb33448bb51))

## [1.1.0](https://github.com/bluefunda/abaper-cli/compare/v1.0.1...v1.1.0) (2026-03-11)


### Features

* initial ABAPer CLI project ([35d3df0](https://github.com/bluefunda/abaper-cli/commit/35d3df0e22c22821f68502c2c11d03913034486c))
* rename binaries to abaper, add man page and Homebrew tap ([#4](https://github.com/bluefunda/abaper-cli/issues/4)) ([748c629](https://github.com/bluefunda/abaper-cli/commit/748c62933a75dfee8a715743f11472fc03805bdb))


### Bug Fixes

* align workflow files with release-foundry patterns ([a86b811](https://github.com/bluefunda/abaper-cli/commit/a86b811e66e81085b5f9e5b7c9beb60dac23570f))
* handle resp.Body.Close() error returns for errcheck lint ([2ec203a](https://github.com/bluefunda/abaper-cli/commit/2ec203af8cd662b4a8065c772cb1cd114f0a5ab1))
* inline CI and release workflows ([59c6b24](https://github.com/bluefunda/abaper-cli/commit/59c6b242e2c4d76a6130eeac305d8eebaccef338))
* push Docker image to bluefunda/abaper and add manual deploy workflow ([5b042c1](https://github.com/bluefunda/abaper-cli/commit/5b042c1db6680093b515f7106e02778fc0831881))
* use GH_PAT for release-please to trigger CI on release PRs ([2645d03](https://github.com/bluefunda/abaper-cli/commit/2645d03212bc0a58a49e56435efcb511c40d302c))

## [1.0.1](https://github.com/bluefunda/abaper-cli/compare/v1.0.0...v1.0.1) (2026-03-11)


### Bug Fixes

* push Docker image to bluefunda/abaper and add manual deploy workflow ([5b042c1](https://github.com/bluefunda/abaper-cli/commit/5b042c1db6680093b515f7106e02778fc0831881))

## 1.0.0 (2026-03-11)


### Features

* initial ABAPer CLI project ([35d3df0](https://github.com/bluefunda/abaper-cli/commit/35d3df0e22c22821f68502c2c11d03913034486c))


### Bug Fixes

* align workflow files with release-foundry patterns ([a86b811](https://github.com/bluefunda/abaper-cli/commit/a86b811e66e81085b5f9e5b7c9beb60dac23570f))
* handle resp.Body.Close() error returns for errcheck lint ([2ec203a](https://github.com/bluefunda/abaper-cli/commit/2ec203af8cd662b4a8065c772cb1cd114f0a5ab1))
* inline CI and release workflows ([59c6b24](https://github.com/bluefunda/abaper-cli/commit/59c6b242e2c4d76a6130eeac305d8eebaccef338))
* use GH_PAT for release-please to trigger CI on release PRs ([2645d03](https://github.com/bluefunda/abaper-cli/commit/2645d03212bc0a58a49e56435efcb511c40d302c))
