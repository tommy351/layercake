extern crate clap;
extern crate layercake;
extern crate slog;
extern crate sloggers;

use clap::{crate_authors, crate_description, crate_name, crate_version, App, Arg, ArgMatches};
use layercake::build::build;
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
    let log = create_logger();
    let config_path = args
        .value_of("config")
        .expect("failed to get config from arguments");
    let config = load_config(config_path.to_string())?;
    build(log, config)
}
