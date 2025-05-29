mod rbln;

use chrono::{Duration, Local};
use clap::Parser;
use log::info;
use rbln::RBLNFeaturesCollector;
use std::thread::sleep;

#[derive(Parser, Debug)]
#[command(version, about = "RBLN NPU Feature Discovery", long_about = None)]
struct CLIArgs {
    #[arg(
        long,
        value_name = "rbln-daemon-url",
        default_value = "http://[::1]:50051",
        help = "Endpoint to RBLN daemon grpc server"
    )]
    rbln_daemon_url: String,

    #[arg(long, help = "Label once and exit")]
    oneshot: bool,

    #[arg(long, help = "Do not add timestamp to the labels")]
    no_timestamp: bool,

    #[arg(
        long,
        default_value_t = 60,
        value_name = "seconds",
        value_parser = clap::value_parser!(u32).range(10..3600),
        help = "Time to sleep between labeling (min: 10s, max: 3600s)"
    )]
    sleep_interval: u32,

    #[arg(
        short = 'o',
        long,
        value_name = "file",
        default_value = "/etc/kubernetes/node-feature-discovery/features.d/rbln-features",
        help = "Path to output file"
    )]
    output_file: String,
}

#[tokio::main]
async fn main() {
    env_logger::init();

    let args = CLIArgs::parse();
    let collector =
        RBLNFeaturesCollector::new(args.rbln_daemon_url, args.output_file, args.no_timestamp).await;

    info!("Start collecting RBLN features");

    // if --oneshot is provided, collect features once and exit
    if args.oneshot {
        collector.collect_features().await;
        return;
    }

    let interval = Duration::seconds(args.sleep_interval as i64);
    let mut next_time = Local::now() + interval;
    loop {
        collector.collect_features().await;
        sleep((next_time - Local::now()).to_std().unwrap());
        next_time += interval;
    }
}
