use config::{Build, BuildScript, Config};
use failure::{Error, Fail};
use serde_derive::{Deserialize, Serialize};
use slog::{debug, info, o, Logger};
use std::collections::HashMap;
use std::fs;
use std::io::Write;
use std::path::Path;
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
  log: Logger,
}

pub fn build(log: Logger, config: Config) -> Result<(), Error> {
  info!(log, "Start building images");

  let temp_dir = tempfile::Builder::new()
    .prefix(".layercake-tmp-")
    .tempdir_in(".")?;

  let builds = sort_builds(&config);

  for (name, build) in builds.iter() {
    let ctx = BuildContext {
      name: name.clone(),
      build,
      dir: TempDir::new_in(temp_dir.path())?,
      tag: build.image.clone().unwrap_or(format!("layercake_{}", name)),
      log: log.new(o!("name" => name.clone())),
    };

    info!(ctx.log, "Building the image");
    build_image(&ctx)?;

    info!(ctx.log, "Saving the last layer");
    save_last_layer(&ctx)?;
  }

  info!(log, "Build successfully");
  Ok(())
}

fn sort_builds(config: &Config) -> Vec<(String, &Build)> {
  let mut names = Vec::new();
  let mut dep_map = (&config.build)
    .into_iter()
    .filter_map(|(name, build)| {
      let deps = pick_build_deps(&build);

      if deps.is_empty() {
        names.push(name);
        None
      } else {
        Some((name, deps))
      }
    })
    .collect::<HashMap<_, _>>();

  while !dep_map.is_empty() {
    dep_map = dep_map
      .into_iter()
      .filter_map(|(name, deps)| {
        let new_deps = deps
          .into_iter()
          .filter(|dep| !names.contains(&dep))
          .collect::<Vec<_>>();

        if new_deps.is_empty() {
          names.push(name);
          None
        } else {
          Some((name, new_deps))
        }
      })
      .collect();
  }

  names
    .into_iter()
    .map(|name| (name.clone(), config.build.get(name).unwrap()))
    .collect::<Vec<_>>()
}

fn pick_build_deps(build: &Build) -> Vec<String> {
  (&build.scripts)
    .into_iter()
    .filter_map(|ref script| match script {
      BuildScript::Import { import } => Some(import.clone()),
      _ => None,
    })
    .collect::<Vec<_>>()
}

fn build_image(ctx: &BuildContext) -> Result<(), Error> {
  let mut temp_file = NamedTempFile::new_in(ctx.dir.path())?;

  {
    let file = temp_file.as_file_mut();

    writeln!(file, "FROM {}", ctx.build.from)?;

    for script in ctx.build.scripts.iter() {
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
        BuildScript::Import { import } => format!(
          "ADD {}/{}.tar /",
          relative_path(ctx.dir.path().parent().unwrap())?
            .to_str()
            .unwrap(),
          import
        ),
      };

      write!(file, "{}\n", line)?;
    }

    file.sync_all()?;
  }

  let mut cmd = Command::new("docker");

  cmd.arg("build");
  cmd.arg("-t").arg(&ctx.tag);
  cmd.arg("-f").arg(temp_file.path());

  for (k, v) in ctx.build.args.iter() {
    cmd.arg("--build-arg").arg(format!("{}={}", k, v));
  }

  cmd.arg(".");

  let mut child = cmd.spawn()?;
  wait_spawn(&mut child)
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

fn save_last_layer(ctx: &BuildContext) -> Result<(), Error> {
  let mut child = Command::new("docker")
    .arg("save")
    .arg(&ctx.tag)
    .stdout(Stdio::piped())
    .spawn()?;

  {
    debug!(ctx.log, "Unpacking the image");
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

  debug!(ctx.log, "Moving the layer");
  fs::rename(src, dst)?;

  wait_spawn(&mut child)
}

fn relative_path(path: &Path) -> Result<&Path, Error> {
  let pwd = std::env::current_dir()?;
  let output = path.strip_prefix(pwd)?;
  Ok(output)
}
