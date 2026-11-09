use zed_extension_api::{self as zed, Command, LanguageServerId, Result};

struct AbapExtension;

impl zed::Extension for AbapExtension {
    fn new() -> Self {
        AbapExtension
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &LanguageServerId,
        worktree: &zed::Worktree,
    ) -> Result<Command> {
        let path = worktree
            .which("abaper")
            .ok_or_else(|| {
                "abaper binary not found in PATH. Install via: brew install bluefunda/tap/abaper"
                    .to_string()
            })?;

        let mut args = vec!["lsp".to_string()];

        // Run in offline mode unless SAP connection is configured
        let has_sap_config = worktree
            .shell_env()
            .iter()
            .any(|(k, _)| k == "SAP_HOST");

        if !has_sap_config {
            args.push("--offline".to_string());
        }

        Ok(Command {
            command: path,
            args,
            env: Default::default(),
        })
    }
}

zed::register_extension!(AbapExtension);
