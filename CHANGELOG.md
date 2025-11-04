# Changelog

# [0.25.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.24.0...back-0.25.0) (2025-11-04)

# [0.24.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.23.0...back-0.24.0) (2025-11-04)


### Bug Fixes

* Refactor reconciliation to batch notifications and prevent infinite loops ([4a1e43f](https://github.com/MohammadBnei/deployment-email-operator/commit/4a1e43fe1793a99fe6f47ea1addc376406be1382))


### Features

* Implement server-side label filtering for deployments ([627739c](https://github.com/MohammadBnei/deployment-email-operator/commit/627739c86913336f73cb31e424573972ef62d8b1))
* Track and alert on changes for multiple deployments individually ([a861e50](https://github.com/MohammadBnei/deployment-email-operator/commit/a861e502b93855ab6721f09169bc402e59d12b74))

# [0.23.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.22.0...back-0.23.0) (2025-11-03)

# [0.22.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.21.0...back-0.22.0) (2025-11-03)

# [0.21.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.20.0...back-0.21.0) (2025-11-03)


### Bug Fixes

* Prevent infinite reconciliation loop by tracking deployment status individually ([0631c7e](https://github.com/MohammadBnei/deployment-email-operator/commit/0631c7e6e0ea0ecbeed333af52f545defdb7f5a8))

# [0.20.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.19.0...back-0.20.0) (2025-11-03)


### Features

* Implement deployment state hashing to prevent duplicate notifications ([53472c0](https://github.com/MohammadBnei/deployment-email-operator/commit/53472c05e50ac7fd5146cec936799a48b1547b44))

# [0.19.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.18.0...back-0.19.0) (2025-11-03)

# [0.18.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.17.0...back-0.18.0) (2025-11-03)


### Features

* Add email template to DeploymentMonitor sample ([05f6df5](https://github.com/MohammadBnei/deployment-email-operator/commit/05f6df50ff0419f6a17fa5bef100484ff59bdd9e))
* Add email templating to DeploymentMonitor ([e085b25](https://github.com/MohammadBnei/deployment-email-operator/commit/e085b2540896f8cbd7632242c19d6f5922aa9753))

# [0.17.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.16.0...back-0.17.0) (2025-11-03)

# [0.16.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.15.0...back-0.16.0) (2025-11-03)

# [0.15.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.14.0...back-0.15.0) (2025-11-03)

# [0.14.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.13.0...back-0.14.0) (2025-11-03)

# [0.13.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.12.0...back-0.13.0) (2025-11-03)

# [0.12.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.11.0...back-0.12.0) (2025-11-03)

# [0.11.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.10.0...back-0.11.0) (2025-11-03)

# [0.10.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.9.0...back-0.10.0) (2025-11-03)

# [0.9.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.8.0...back-0.9.0) (2025-11-03)

# [0.8.0](https://github.com/MohammadBnei/deployment-email-operator/compare/back-0.7.0...back-0.8.0) (2025-11-03)


### Bug Fixes

* Correctly apply annotations to Deployment metadata ([019097f](https://github.com/MohammadBnei/deployment-email-operator/commit/019097f50b89b38aeb7cd1c35e4320914e23567a))

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
