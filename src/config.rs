use failure::Error;
use log::*;
use serde_derive::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs::File;

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Config {
    #[serde(skip_serializing_if = "HashMap::is_empty")]
    pub build: HashMap<String, Build>,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
pub struct Build {
    pub from: String,
    pub image: Option<String>,
    pub scripts: Vec<BuildScript>,
    #[serde(skip_serializing_if = "HashMap::is_empty", default = "HashMap::new")]
    pub args: HashMap<String, String>,
}

#[derive(Debug, Serialize, Deserialize, PartialEq)]
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
    Cmd { cmd: String },
    Maintainer { maintainer: String },
    OnBuild { onbuild: String },
    StopSignal { stopsignal: String },
    HealthCheck { healthcheck: String },
    Shell { shell: String },
    Import { import: String },
}

pub fn load_config(path: String) -> Result<Config, Error> {
    debug!("Reading config from: {}", path);
    let file = File::open(path)?;
    let config: Config = serde_yaml::from_reader(file)?;
    Ok(config)
}

#[cfg(test)]
mod tests {
    use super::*;
    use failure::Error;
    use std::collections::HashMap;
    use std::io::Write;
    use tempfile::NamedTempFile;

    #[test]
    fn test_load_config() -> Result<(), Error> {
        let mut file = NamedTempFile::new()?;
        write!(
            file,
            r#"
build:
    foo:
        from: busybox
        image: foo_img
        args:
            foo: bar
        scripts:
        - run: a
        - arg: b
        - workdir: c
        - env: d
        - label: e
        - expose: f
        - add: g
        - copy: h
        - entrypoint: i
        - volume: j
        - user: k
        - cmd: l
        - maintainer: m
        - onbuild: n
        - stopsignal: o
        - healthcheck: p
        - shell: q
        - import: r
"#
        )?;

        let config = load_config(String::from(file.path().to_str().unwrap()))?;
        let mut builds = HashMap::new();

        builds.insert(
            "foo".to_string(),
            Build {
                from: "busybox".to_string(),
                image: Some("foo_img".to_string()),
                scripts: vec![
                    BuildScript::Run {
                        run: "a".to_string(),
                    },
                    BuildScript::Arg {
                        arg: "b".to_string(),
                    },
                    BuildScript::WorkDir {
                        workdir: "c".to_string(),
                    },
                    BuildScript::Env {
                        env: "d".to_string(),
                    },
                    BuildScript::Label {
                        label: "e".to_string(),
                    },
                    BuildScript::Expose {
                        expose: "f".to_string(),
                    },
                    BuildScript::Add {
                        add: "g".to_string(),
                    },
                    BuildScript::Copy {
                        copy: "h".to_string(),
                    },
                    BuildScript::Entrypoint {
                        entrypoint: "i".to_string(),
                    },
                    BuildScript::Volume {
                        volume: "j".to_string(),
                    },
                    BuildScript::User {
                        user: "k".to_string(),
                    },
                    BuildScript::Cmd {
                        cmd: "l".to_string(),
                    },
                    BuildScript::Maintainer {
                        maintainer: "m".to_string(),
                    },
                    BuildScript::OnBuild {
                        onbuild: "n".to_string(),
                    },
                    BuildScript::StopSignal {
                        stopsignal: "o".to_string(),
                    },
                    BuildScript::HealthCheck {
                        healthcheck: "p".to_string(),
                    },
                    BuildScript::Shell {
                        shell: "q".to_string(),
                    },
                    BuildScript::Import {
                        import: "r".to_string(),
                    },
                ],
                args: [("foo".to_string(), "bar".to_string())]
                    .iter()
                    .cloned()
                    .collect::<HashMap<_, _>>(),
            },
        );

        assert_eq!(config, Config { build: builds });
        Ok(())
    }
}
