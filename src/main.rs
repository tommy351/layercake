extern crate clap;
extern crate layercake;
extern crate slog;
extern crate sloggers;

use clap::{crate_authors, crate_description, crate_name, crate_version, App, Arg, ArgMatches};
use layercake::build::Builder;
use layercake::config::load_config;
use slog::Logger;

fn main() {
    let args = App::new(crate_name!())
        .version(crate_version!())
        .author(crate_authors!())
        .about(crate_description!())
        .arg(
            Arg::with_name("config")
                .short("c")
                .long("config")
                .value_name("FILE")
                .help("Sets the path of the config file")
                .default_value("layercake.yml")
                .takes_value(true),
        )
        .arg(
            Arg::with_name("build-arg")
                .long("build-arg")
                .value_name("key=val")
                .help("Sets build-time variables")
                .multiple(true)
                .takes_value(true),
        )
        .arg(
            Arg::with_name("compress")
                .long("compress")
                .help("Compresses the build context using gzip"),
        )
        .arg(
            Arg::with_name("force-rm")
                .long("force-rm")
                .help("Always remove intermediate containers"),
        )
        .arg(
            Arg::with_name("pull")
                .long("pull")
                .help("Always attempt to pull a newer version of the image"),
        )
        .arg(
            Arg::with_name("no-cache")
                .long("no-cache")
                .help("Do not use cache when building the image"),
        )
        .arg(
            Arg::with_name("dry-run")
                .long("dry-run")
                .help("Prints the content of Dockerfile without building images"),
        )
        .get_matches();

    match run(args) {
        Ok(_) => {}
        Err(e) => {
            eprintln!("error: {:?}", e);
            std::process::exit(1);
        }
    }
}

fn create_logger() -> Logger {
    use sloggers::terminal::{Destination, TerminalLoggerBuilder};
    use sloggers::types::{Format, Severity, SourceLocation};
    use sloggers::Build;

    let mut builder = TerminalLoggerBuilder::new();
    builder.format(Format::Full);
    builder.level(Severity::Debug);
    builder.destination(Destination::Stderr);
    builder.source_location(SourceLocation::None);

    builder.build().expect("failed to build a logger")
}

fn run(args: ArgMatches) -> Result<(), failure::Error> {
    let config_path = args
        .value_of("config")
        .expect("failed to get config from arguments");

    let builder = Builder {
        log: create_logger(),
        config: load_config(config_path.to_string())?,
        args: args
            .values_of("build-arg")
            .unwrap_or_default()
            .map(|s| s.to_string())
            .collect(),
        compress: args.is_present("compress"),
        force_rm: args.is_present("force-rm"),
        pull: args.is_present("pull"),
        no_cache: args.is_present("no-cache"),
        dry_run: args.is_present("dry-run"),
    };

    builder.build()
}
