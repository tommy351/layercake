extern crate clap;
extern crate layercake;
extern crate slog;
extern crate sloggers;

use clap::{crate_authors, crate_description, crate_name, crate_version, App, Arg, ArgMatches};
use layercake::build::build;
use layercake::config::load_config;
use slog::Logger;
use sloggers::terminal::{Destination, TerminalLoggerBuilder};
use sloggers::types::Severity;
use sloggers::Build;

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
        .get_matches();

    let mut builder = TerminalLoggerBuilder::new();
    builder.level(Severity::Debug);
    builder.destination(Destination::Stderr);

    let log = builder.build().unwrap();

    match run(log.clone(), args) {
        Ok(_) => {}
        Err(e) => {
            eprintln!("error: {:?}", e);
            std::process::exit(1);
        }
    }
}

fn run(log: Logger, args: ArgMatches) -> Result<(), failure::Error> {
    let config_path = args.value_of("config").unwrap();
    let config = load_config(config_path.to_string())?;
    build(log, config)
}
