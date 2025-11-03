# Changelog

# [0.7.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.6.0...back-0.7.0) (2025-11-03)

# [0.6.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.5.0...back-0.6.0) (2025-11-03)

# [0.5.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.4.0...back-0.5.0) (2025-11-03)

# [0.4.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.3.0...back-0.4.0) (2025-11-03)

# [0.3.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.2.0...back-0.3.0) (2025-11-03)

# [0.2.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.1.0...back-0.2.0) (2025-11-03)

# 0.1.0 (2025-11-03)


### Bug Fixes

* Correct SMTP server field name in email sending ([a9455f2](https://github.com/MohammadBnei/deployment-email-operator/commit/a9455f2f251212d88ccc353894a7c061460fe402))
* Handle nil SMTPConfigSecretRef to prevent nil pointer dereference ([bf89d1c](https://github.com/MohammadBnei/deployment-email-operator/commit/bf89d1c04876b423756c7794b7ecf1e5cf5af968))
* Prevent nil pointer dereference when accessing deployment image or replicas ([4bb971e](https://github.com/MohammadBnei/deployment-email-operator/commit/4bb971e1a18357aaa24f0f5ec38d58f7b5154905))


### Features

* externalize full SMTP config to a secret ([0341098](https://github.com/MohammadBnei/deployment-email-operator/commit/03410989f515f5f50f9e27d2e8efdd8a7a830d07))
* Implement core reconciliation logic and email sending for DeploymentMonitor ([efa010f](https://github.com/MohammadBnei/deployment-email-operator/commit/efa010f3900cf4949d340c509801a12797b83f94))
