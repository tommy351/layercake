extern crate failure;
extern crate layercake;
extern crate log;
extern crate simplelog;
extern crate structopt;

use failure::Error;
use layercake::build::Builder;
use layercake::config::load_config;
use log::*;
use simplelog::{Config, TermLogger};
use structopt::StructOpt;

#[derive(Debug, StructOpt)]
struct Opt {
    #[structopt(
        short = "c",
        long = "config",
        default_value = "layercake.yml",
        help = "Sets the path of the config file"
    )]
    config_path: String,
    #[structopt(long = "build-arg", help = "Sets build-time variables")]
    build_args: Vec<String>,
    #[structopt(long, help = "Compresses the build context using gzip")]
    compress: bool,
    #[structopt(long = "force-rm", help = "Always remove intermediate containers")]
    force_rm: bool,
    #[structopt(
        long = "pull",
        help = "Always attempt to pull a newer version of the image"
    )]
    pull: bool,
    #[structopt(long = "no-cache", help = "Do not use cache when building the image")]
    no_cache: bool,
    #[structopt(
        long = "dry-run",
        help = "Prints the content of Dockerfile without building images"
    )]
    dry_run: bool,
    #[structopt(long = "log-level", help = "Sets log level", default_value = "info")]
    log_level: LevelFilter,
}

fn main() -> Result<(), Error> {
    let opt = Opt::from_args();

    TermLogger::init(opt.log_level, Config::default())?;

    let builder = Builder {
        config: load_config(opt.config_path)?,
        args: opt.build_args,
        compress: opt.compress,
        force_rm: opt.force_rm,
        pull: opt.pull,
        no_cache: opt.no_cache,
        dry_run: opt.dry_run,
    };

    builder.build()
}
