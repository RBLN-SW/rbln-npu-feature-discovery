use anyhow::{anyhow, Result};
use chrono::{Duration, Local};
use log::{debug, error};
use std::{fs, path::PathBuf};

mod rblnservices {
    include!("./rblnservices.rs");
}

#[derive(Debug, Clone, strum_macros::Display)]
enum RBLNDeviceFamily {
    ATOM,
    REBEL,
}

#[derive(Debug, Clone, strum_macros::Display)]
enum RBLNDeviceProduct {
    CA02,
    CA12,
    CA15,
    CA22,
    CA25,
}
impl RBLNDeviceProduct {
    pub fn to_feature_string(&self) -> String {
        format!("RBLN-{}", self.to_string())
    }
    pub fn from_device_id(id: &str) -> Result<Self> {
        match id {
            "1020" => Ok(RBLNDeviceProduct::CA02), // CA02
            "1021" => Ok(RBLNDeviceProduct::CA02), // CA02 with SR-IOV
            "1120" => Ok(RBLNDeviceProduct::CA12), // CA12
            "1121" => Ok(RBLNDeviceProduct::CA02), // CA12 with SR-IOV
            "1150" => Ok(RBLNDeviceProduct::CA15), // CA15
            "1220" => Ok(RBLNDeviceProduct::CA22), // CA22
            "1221" => Ok(RBLNDeviceProduct::CA22), // CA22 with SR-IOV
            "1250" => Ok(RBLNDeviceProduct::CA25), // CA25
            _ => Err(anyhow!("Unknown device id: {}", id)),
        }
    }
    pub fn family(&self) -> Result<RBLNDeviceFamily> {
        let self_str = self.to_string();
        if self_str.starts_with("CA") {
            Ok(RBLNDeviceFamily::ATOM)
        } else if self_str.starts_with("CR") {
            Ok(RBLNDeviceFamily::REBEL)
        } else {
            Err(anyhow!("Unknown product name: {}", self_str))
        }
    }
}

#[derive(Debug, Clone)]
struct RBLNFeatures {
    npu_present: bool,
    npu_count: Option<u8>,
    npu_family: Option<String>,
    npu_product: Option<String>,
    driver_version_full: Option<String>,
    driver_version_major: Option<String>,
    driver_version_minor: Option<String>,
    driver_version_patch: Option<String>,
    driver_version_revision: Option<String>,
}
impl RBLNFeatures {
    pub fn new() -> Self {
        Self {
            npu_present: false,
            npu_count: None,
            npu_family: None,
            npu_product: None,
            driver_version_full: None,
            driver_version_major: None,
            driver_version_minor: None,
            driver_version_patch: None,
            driver_version_revision: None,
        }
    }

    pub fn to_plain_text(&self) -> String {
        let label_prefix = "rebellions.ai";
        let mut val = "".to_owned();
        val.push_str(format!("{}/npu.present={}\n", label_prefix, self.npu_present).as_str());
        if let Some(npu_count) = &self.npu_count {
            val.push_str(format!("{}/npu.count={}\n", label_prefix, npu_count).as_str());
        }
        if let Some(npu_family) = &self.npu_family {
            val.push_str(format!("{}/npu.family={}\n", label_prefix, npu_family).as_str());
        }
        if let Some(npu_product) = &self.npu_product {
            val.push_str(format!("{}/npu.product={}\n", label_prefix, npu_product).as_str());
        }
        if let Some(driver_version_full) = &self.driver_version_full {
            val.push_str(
                format!(
                    "{}/driver-version.full={}\n",
                    label_prefix, driver_version_full
                )
                .as_str(),
            );
        }
        if let Some(driver_version_major) = &self.driver_version_major {
            val.push_str(
                format!(
                    "{}/driver-version.major={}\n",
                    label_prefix, driver_version_major
                )
                .as_str(),
            );
        }
        if let Some(driver_version_minor) = &self.driver_version_minor {
            val.push_str(
                format!(
                    "{}/driver-version.minor={}\n",
                    label_prefix, driver_version_minor
                )
                .as_str(),
            );
        }
        if let Some(driver_version_patch) = &self.driver_version_patch {
            val.push_str(
                format!(
                    "{}/driver-version.patch={}\n",
                    label_prefix, driver_version_patch
                )
                .as_str(),
            );
        }
        if let Some(driver_version_revision) = &self.driver_version_revision {
            val.push_str(
                format!(
                    "{}/driver-version.revision={}\n",
                    label_prefix, driver_version_revision
                )
                .as_str(),
            );
        }
        val
    }
}

pub struct RBLNFeaturesCollector {
    rbln_daemon_url: String,
    output_file: PathBuf,
    no_timestamp: bool,
}
impl RBLNFeaturesCollector {
    pub async fn new(rbln_daemon_url: String, output_file: String, no_timestamp: bool) -> Self {
        Self {
            rbln_daemon_url,
            output_file: PathBuf::from(output_file),
            no_timestamp,
        }
    }

    pub async fn collect_features(&self) {
        let mut features = RBLNFeatures::new();

        let daemon_client_result = rblnservices::rbln_services_client::RblnServicesClient::connect(
            self.rbln_daemon_url.clone(),
        )
        .await;

        if daemon_client_result.is_err()
            || self
                ._collect_features_from_daemon(&mut features, &mut daemon_client_result.unwrap())
                .await
                .is_err()
        {
            // fallback to collecting features from sysfs if either
            // 1. daemon does not exist or
            // 2. failed to get features from daemon
            debug!("Failed to collect features from daemon. Fallback to collecting from sysfs..");
            self._collect_features_from_sysfs(&mut features).unwrap();
        }

        self.save_features(&features).unwrap();
    }

    async fn _collect_features_from_daemon(
        &self,
        features: &mut RBLNFeatures,
        daemon_client: &mut rblnservices::rbln_services_client::RblnServicesClient<
            tonic::transport::Channel,
        >,
    ) -> Result<()> {
        // Collect serviceable devices from daemon
        let mut devices = Vec::<rblnservices::Device>::new();
        match daemon_client
            .get_serviceable_device_list(tonic::Request::new(rblnservices::Empty {}))
            .await
        {
            Ok(mut resp) => {
                while let Some(device) = resp.get_mut().message().await.unwrap() {
                    devices.push(device);
                }
            }
            Err(e) => {
                let error: &str;
                if e.code() == tonic::Code::Unimplemented {
                    error = "getServiceableDeviceList is not implemented";
                    debug!("{}", error);
                } else {
                    error = "Internal error in getServiceableDeviceList";
                    error!("{}", error);
                }
                return Err(anyhow!(error));
            }
        }

        if devices.len() > 0 {
            features.npu_present = true;
            features.npu_count = Some(devices.len() as u8);

            // NOTE: This assumes that there is no possibility of heterogeneous RBLN devices equipped on the same machine.
            // If this situation can happen, it's necessary to determine the logics of which product should be used as features.
            let first_device = devices.first().unwrap();
            let first_product =
                RBLNDeviceProduct::from_device_id(first_device.dev_id.as_str()).unwrap();
            features.npu_product = Some(first_product.to_feature_string());
            features.npu_family = Some(first_product.family().unwrap().to_string());

            // call getVersion using the first device
            if let Ok(resp) = daemon_client
                .get_version(tonic::Request::new(first_device.clone()))
                .await
            {
                let driver_version = resp.get_ref().drv_version.clone();
                let [semver, revision] = driver_version.split("-").collect::<Vec<&str>>()[0..2]
                else {
                    return Err(anyhow!(
                        "Failed to split driver version with hyphens: {}",
                        driver_version
                    ));
                };
                features.driver_version_full = Some(semver.to_string());
                features.driver_version_revision = Some(revision.to_string());

                let [semver_major, semver_minor, semver_patch] =
                    semver.split(".").collect::<Vec<&str>>()[0..3]
                else {
                    return Err(anyhow!("Failed to split semver with dots: {}", semver));
                };
                features.driver_version_major = Some(semver_major.to_string());
                features.driver_version_minor = Some(semver_minor.to_string());
                features.driver_version_patch = Some(semver_patch.to_string());
            }
        }

        Ok(())
    }

    fn _collect_features_from_sysfs(&self, features: &mut RBLNFeatures) -> Result<()> {
        //////////////////////////////
        // Collect "npu.*" features //
        //////////////////////////////
        let rbln_vendor_id = "0x1eff";
        let pci_devices_path = "/sys/bus/pci/devices";
        let mut rbln_products = Vec::<RBLNDeviceProduct>::new();
        for entry in fs::read_dir(pci_devices_path)? {
            let entry = entry?;
            let device_path = entry.path();

            let vendor_file = device_path.join("vendor");
            let vendor_id = fs::read_to_string(vendor_file)?.trim().to_string();
            if vendor_id == rbln_vendor_id {
                // do not count PF if SR-IOV is enabled
                let sriov_numvfs_path = device_path.join("sriov_numvfs");
                if sriov_numvfs_path.exists() {
                    let numvfs = fs::read_to_string(sriov_numvfs_path)?
                        .trim()
                        .parse::<u8>()
                        .unwrap();
                    if numvfs != 0 {
                        continue;
                    }
                }

                let device_id_path = device_path.join("device");
                let device_id = fs::read_to_string(device_id_path)?.trim().replace("0x", "");
                let product = RBLNDeviceProduct::from_device_id(device_id.as_str()).unwrap();
                rbln_products.push(product);
            }
        }
        if rbln_products.len() > 0 {
            features.npu_present = true;
            features.npu_count = Some(rbln_products.len() as u8);

            // NOTE: This assumes that there is no possibility of heterogeneous RBLN devices equipped on the same machine.
            // If this situation can happen, it's necessary to determine the logics of which product should be used as features.
            let product = rbln_products.first().unwrap();
            features.npu_product = Some(product.to_feature_string());
            features.npu_family = Some(product.family().unwrap().to_string());
        }

        /////////////////////////////////////////
        // Collect "driver-version.*" features //
        /////////////////////////////////////////
        let sysfs_path = PathBuf::from("/sys/class/rebellions");
        if !sysfs_path.exists() {
            // This can happen when there are RBLN devices but no drivers installed.
            debug!("rebellions sysfs not found");
            return Ok(());
        }
        let sysfs_first_device = sysfs_path.join("rbln0");
        if !sysfs_first_device.exists() {
            error!("sysfs for device rbln0 not found");
            return Ok(());
        }
        let kernel_version_file = sysfs_first_device.join("kernel_version");
        if !kernel_version_file.exists() {
            error!("kernel_version file not found in sysfs");
            return Ok(());
        }

        let driver_version = fs::read_to_string(kernel_version_file)?.trim().to_string();
        let [semver, revision] = driver_version.split("-").collect::<Vec<&str>>()[0..2] else {
            error!(
                "Failed to split driver version with hyphens: {}",
                driver_version
            );
            return Ok(());
        };
        features.driver_version_full = Some(semver.to_string());
        features.driver_version_revision = Some(revision.to_string());

        let [semver_major, semver_minor, semver_patch] =
            semver.split(".").collect::<Vec<&str>>()[0..3]
        else {
            error!("Failed to split semver with dots: {}", semver);
            return Ok(());
        };
        features.driver_version_major = Some(semver_major.to_string());
        features.driver_version_minor = Some(semver_minor.to_string());
        features.driver_version_patch = Some(semver_patch.to_string());

        Ok(())
    }

    fn save_features(&self, features: &RBLNFeatures) -> Result<()> {
        let features_path = self.output_file.clone();
        let mut text_value = features.to_plain_text();

        // write expiry time to one hour later
        if !self.no_timestamp {
            let one_hour_later = (Local::now() + Duration::hours(1))
                .to_rfc3339_opts(chrono::SecondsFormat::Secs, false);
            text_value = format!("# +expiry-time={}\n{}", one_hour_later, text_value);
        }

        debug!("Collected features:\n{text_value}");

        let features_parent_path = features_path.parent().unwrap();
        if features_parent_path.exists() && !text_value.is_empty() {
            let filename = features_path.file_name().unwrap().to_str().unwrap();
            // The guide suggests writing into a temporary file and atomically create/update the original file by doing a file rename operation.
            // https://kubernetes-sigs.github.io/node-feature-discovery/v0.17/usage/customization-guide.html#local-feature-source
            let features_temp_path = features_parent_path.join(format!(".{}", filename));
            fs::write(&features_temp_path, text_value)?;
            fs::rename(features_temp_path, features_path)?;
        }
        Ok(())
    }
}
