use config::{Build, BuildScript, Config};
use failure::{Error, Fail};
use log::*;
use serde_derive::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};
use std::fs;
use std::io::Write;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use tar::Archive;
use tempfile::{NamedTempFile, TempDir};

#[derive(Debug, Fail)]
enum BuildError {
    #[fail(display = "command exit with status code: {}", code)]
    CommandExit { code: i32 },
}

#[derive(Debug, Serialize, Deserialize)]
struct DockerImageRepository {
    latest: String,
}

#[derive(Debug)]
struct BuildContext<'a> {
    name: String,
    build: &'a Build,
    dir: TempDir,
    tag: String,
}

#[derive(Debug)]
pub struct Builder {
    pub config: Config,
    pub args: Vec<String>,
    pub compress: bool,
    pub force_rm: bool,
    pub pull: bool,
    pub no_cache: bool,
    pub dry_run: bool,
    pub current_dir: PathBuf,
}

impl Builder {
    pub fn build(&self) -> Result<(), Error> {
        let temp_dir = tempfile::Builder::new()
            .prefix(".layercake-tmp")
            .rand_bytes(0)
            .tempdir_in(&self.current_dir)?;

        let dependency_map = (&self.config.build)
            .into_iter()
            .map(|(name, build)| {
                (
                    name,
                    (&build.scripts)
                        .into_iter()
                        .filter_map(|ref script| match script {
                            BuildScript::Import { import } => Some(import),
                            _ => None,
                        })
                        .collect::<HashSet<_>>(),
                )
            })
            .collect::<HashMap<_, _>>();

        let dependant_map = (&self.config.build)
            .into_iter()
            .map(|(name, _)| {
                (
                    name,
                    (&dependency_map)
                        .into_iter()
                        .filter(|(_, deps)| deps.contains(name))
                        .map(|(&name, _)| name)
                        .collect::<HashSet<_>>(),
                )
            })
            .collect::<HashMap<_, _>>();

        let mut done_builds: HashSet<&String> = HashSet::new();

        while done_builds.len() != self.config.build.len() {
            for (name, build) in &self.config.build {
                if done_builds.contains(name) {
                    continue;
                }

                let dependencies = dependency_map.get(name).unwrap();
                let undone_deps = dependencies
                    .into_iter()
                    .filter(|x| !done_builds.contains(*x))
                    .collect::<Vec<_>>();

                if !undone_deps.is_empty() {
                    debug!("Skip {} because it depends on: {:?}", name, undone_deps);
                    continue;
                }

                let dependants = dependant_map.get(name).unwrap();
                let ctx = BuildContext {
                    name: name.clone(),
                    build,
                    dir: TempDir::new_in(temp_dir.path())?,
                    tag: build.image.clone().unwrap_or(format!("layercake_{}", name)),
                };

                if self.dry_run {
                    info!("Dockerfile: {}", name);
                    self.write_docker_file(&ctx, &mut std::io::stdout())?;
                } else {
                    info!("Building the image: {}", name);
                    self.build_image(&ctx)?;

                    if !dependants.is_empty() {
                        info!("Saving the last layer: {}", name);
                        self.save_last_layer(&ctx)?;
                    }
                }

                done_builds.insert(name);
            }
        }

        Ok(())
    }

    fn build_image(&self, ctx: &BuildContext) -> Result<(), Error> {
        let mut file = NamedTempFile::new_in(ctx.dir.path())?;

        {
            let mut f = file.as_file_mut();
            self.write_docker_file(ctx, &mut f)?;
            f.sync_all()?;
        }

        let mut cmd = Command::new("docker");

        cmd.arg("build");
        cmd.arg("-t").arg(&ctx.tag);
        cmd.arg("-f").arg(file.path());

        for arg in &self.args {
            cmd.arg("--build-arg").arg(arg);
        }

        if self.compress {
            cmd.arg("--compress");
        }

        if self.force_rm {
            cmd.arg("--force-rm");
        }

        if self.pull {
            cmd.arg("--pull");
        }

        if self.no_cache {
            cmd.arg("--no-cache");
        }

        for (k, v) in &ctx.build.args {
            cmd.arg("--build-arg").arg(format!("{}={}", k, v));
        }

        cmd.arg(self.current_dir.as_os_str());

        let mut child = cmd.spawn()?;
        wait_spawn(&mut child)
    }

    fn write_docker_file<W: Write>(&self, ctx: &BuildContext, file: &mut W) -> Result<(), Error> {
        writeln!(file, "FROM {}", ctx.build.from)?;

        for script in &ctx.build.scripts {
            let line = match script {
                BuildScript::Run { run } => format!("RUN {}", run),
                BuildScript::Arg { arg } => format!("ARG {}", arg),
                BuildScript::WorkDir { workdir } => format!("WORKDIR {}", workdir),
                BuildScript::Env { env } => format!("ENV {}", env),
                BuildScript::Label { label } => format!("LABEL {}", label),
                BuildScript::Expose { expose } => format!("EXPOSE {}", expose),
                BuildScript::Add { add } => format!("ADD {}", add),
                BuildScript::Copy { copy } => format!("COPY {}", copy),
                BuildScript::Entrypoint { entrypoint } => format!("ENTRYPOINT {}", entrypoint),
                BuildScript::Volume { volume } => format!("VOLUME {}", volume),
                BuildScript::User { user } => format!("USER {}", user),
                BuildScript::Cmd { cmd } => format!("CMD {}", cmd),
                BuildScript::Maintainer { maintainer } => format!("MAINTAINER {}", maintainer),
                BuildScript::OnBuild { onbuild } => format!("ONBUILD {}", onbuild),
                BuildScript::StopSignal { stopsignal } => format!("STOPSIGNAL {}", stopsignal),
                BuildScript::HealthCheck { healthcheck } => format!("HEALTHCHECK {}", healthcheck),
                BuildScript::Shell { shell } => format!("SHELL {}", shell),
                BuildScript::Import { import } => format!(
                    "ADD {}/{}.tar /",
                    ctx.dir
                        .path()
                        .parent()
                        .unwrap()
                        .strip_prefix(&self.current_dir)
                        .unwrap()
                        .to_str()
                        .unwrap(),
                    import
                ),
            };

            write!(file, "{}\n", line)?;
        }

        Ok(())
    }

    fn save_last_layer(&self, ctx: &BuildContext) -> Result<(), Error> {
        let mut child = Command::new("docker")
            .arg("save")
            .arg(&ctx.tag)
            .stdout(Stdio::piped())
            .spawn()?;

        {
            debug!("Unpacking the image");
            let stdout = child.stdout.as_mut().unwrap();
            let mut archive = Archive::new(stdout);
            archive.unpack(ctx.dir.path())?;
        }

        let repo_path = ctx.dir.path().join("repositories");
        let file = fs::File::open(repo_path)?;
        let repo_map: HashMap<String, DockerImageRepository> = serde_json::from_reader(file)?;
        let (_, repo) = repo_map.iter().next().unwrap();
        let src = ctx.dir.path().join(&repo.latest).join("layer.tar");
        let dst = ctx
            .dir
            .path()
            .parent()
            .unwrap()
            .join(format!("{}.tar", ctx.name));

        debug!("Moving the layer");
        fs::rename(src, dst)?;

        wait_spawn(&mut child)
    }
}

fn wait_spawn(child: &mut Child) -> Result<(), Error> {
    let status = child.wait()?;

    if status.success() {
        Ok(())
    } else {
        Err(Error::from(BuildError::CommandExit {
            code: status.code().unwrap(),
        }))
    }
}
