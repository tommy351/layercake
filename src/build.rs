use config::{BuildConfig, BuildScript, BuildStep};
use failure::{Error, Fail};
use serde_derive::{Deserialize, Serialize};
use slog::{debug, info, o, Logger};
use std::collections::HashMap;
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use tar::Archive;

#[derive(Debug, Fail)]
enum BuildError {
  #[fail(display = "command exit with status code: {}", code)]
  CommandExit { code: i32 },
}

#[derive(Debug, Serialize, Deserialize)]
struct DockerImageRepository {
  latest: String,
}

pub fn build(log: Logger, config: BuildConfig) -> Result<(), Error> {
  info!(log, "Start building images");

  for (name, step) in config.steps.iter() {
    let tag = step.image.clone().unwrap_or(format!("layercake_{}", name));
    let step_log = log.new(o!("step" => name.clone()));

    info!(step_log, "Building the image");
    build_image(&step_log, &step, &tag)?;

    info!(step_log, "Saving the last layer");
    save_last_layer(&step_log, &name, &tag)?;
  }

  Ok(())
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

fn build_image(log: &Logger, step: &BuildStep, tag: &String) -> Result<(), Error> {
  let mut cmd = Command::new("docker");

  cmd.stdin(Stdio::piped());

  cmd.arg("build");
  cmd.arg("-t");
  cmd.arg(tag);
  cmd.args(&["-f", "-"]);

  for (k, v) in step.args.iter() {
    cmd.arg("--build-arg");
    cmd.arg(format!("{}={}", k, v));
  }

  cmd.arg(".");

  let mut child = cmd.spawn()?;

  debug!(log, "Writing Dockerfile");
  write_docker_file(&mut child, step)?;

  wait_spawn(&mut child)
}

fn write_docker_file(child: &mut Child, step: &BuildStep) -> Result<(), Error> {
  let stdin = child.stdin.as_mut().unwrap();

  stdin.write(format!("FROM {}\n", step.from).as_bytes())?;

  for script in step.scripts.iter() {
    match script {
      BuildScript::Run { run } => stdin.write(format!("RUN {}\n", run).as_bytes())?,
      BuildScript::Arg { arg } => stdin.write(format!("ARG {}\n", arg).as_bytes())?,
      BuildScript::WorkDir { workdir } => {
        stdin.write(format!("WORKDIR {}\n", workdir).as_bytes())?
      }
      BuildScript::Env { env } => stdin.write(format!("ENV {}\n", env).as_bytes())?,
      BuildScript::Label { label } => stdin.write(format!("LABEL {}\n", label).as_bytes())?,
      BuildScript::Expose { expose } => stdin.write(format!("EXPOSE {}\n", expose).as_bytes())?,
      BuildScript::Add { add } => stdin.write(format!("ADD {}\n", add).as_bytes())?,
      BuildScript::Copy { copy } => stdin.write(format!("COPY {}\n", copy).as_bytes())?,
      BuildScript::Entrypoint { entrypoint } => {
        stdin.write(format!("ENTRYPOINT {}\n", entrypoint).as_bytes())?
      }
      BuildScript::Volume { volume } => stdin.write(format!("VOLUME {}\n", volume).as_bytes())?,
      BuildScript::User { user } => stdin.write(format!("USER {}\n", user).as_bytes())?,
      BuildScript::Import { import } => {
        stdin.write(format!("ADD .layercake-tmp/{}.tar /\n", import).as_bytes())?
      }
    };
  }

  Ok(())
}

fn save_last_layer(log: &Logger, name: &String, tag: &String) -> Result<(), Error> {
  let temp_dir = Path::new(".layercake-tmp");
  let image_dir = temp_dir.join(name);

  debug!(log, "Creating a temporary directory"; "path" => image_dir.to_str());
  fs::create_dir_all(&image_dir)?;

  let mut child = Command::new("docker")
    .arg("save")
    .arg(tag)
    .stdout(Stdio::piped())
    .spawn()?;

  debug!(log, "Unpacking the image"; "path" => image_dir.to_str());
  unpack_image(&mut child, &image_dir)?;

  let last_layer = get_last_layer(&image_dir)?;
  let src = image_dir.join(last_layer).join("layer.tar");
  let dst = temp_dir.join(format!("{}.tar", name));
  debug!(log, "Moving the layer"; "from" => src.to_str(), "to" => dst.to_str());
  fs::rename(src, dst)?;

  debug!(log, "Cleanup the temporary directory"; "path" => image_dir.to_str());
  fs::remove_dir_all(image_dir)?;

  wait_spawn(&mut child)
}

fn unpack_image<P: AsRef<Path>>(child: &mut Child, path: P) -> Result<(), Error> {
  let stdout = child.stdout.as_mut().unwrap();
  let mut archive = Archive::new(stdout);
  archive.unpack(path)?;
  Ok(())
}

fn get_last_layer(path: &PathBuf) -> Result<String, Error> {
  let repo_path = Path::new(path).join("repositories");
  let file = fs::File::open(repo_path)?;
  let repo_map: HashMap<String, DockerImageRepository> = serde_json::from_reader(file)?;
  let (_, repo) = repo_map.iter().next().unwrap();
  Ok(repo.latest.clone())
}

fn pick_step_deps(scripts: Vec<BuildScript>) -> Vec<String> {
  scripts
    .into_iter()
    .filter_map(|script| match script {
      BuildScript::Import { import } => Some(import.clone()),
      _ => None,
    })
    .collect::<Vec<_>>()
}
