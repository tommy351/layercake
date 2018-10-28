use failure::Error;
use serde_derive::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs::File;

#[derive(Debug, Serialize, Deserialize)]
pub struct BuildConfig {
  #[serde(skip_serializing_if = "HashMap::is_empty")]
  pub steps: HashMap<String, BuildStep>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct BuildStep {
  pub from: String,
  pub image: Option<String>,
  pub scripts: Vec<BuildScript>,
  #[serde(skip_serializing_if = "HashMap::is_empty", default = "HashMap::new")]
  pub args: HashMap<String, String>,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(untagged)]
pub enum BuildScript {
  Run { run: String },
  Arg { arg: String },
  WorkDir { workdir: String },
  Env { env: String },
  Label { label: String },
  Expose { expose: String },
  Add { add: String },
  Copy { copy: String },
  Entrypoint { entrypoint: String },
  Volume { volume: String },
  User { user: String },
  Import { import: String },
}

pub fn load_config(path: String) -> Result<BuildConfig, Error> {
  let file = File::open(path)?;
  let config: BuildConfig = serde_yaml::from_reader(file)?;
  Ok(config)
}
