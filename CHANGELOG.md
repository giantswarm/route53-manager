# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project's packages adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Add  EC2 ENI filter to filter out CAPI based ENIs. THis is encessery during migration.

## [1.4.0] - 2024-05-02

### Added

- Add global.podSecurityStandards.enforced value for PSS migration.

### Changed

- Set up securityContext to comply with PSS policies.

## [1.3.0] - 2022-01-13

### Changed

- Update Batch API version from `v1beta1` to `v1`.
- Sort all ENIs by the name before creating DNS record, to avoid wrong IP order.

## [1.2.0] - 2021-06-03

### Changed

- Always created `etcd0` record to avoid issues in China.

## [1.1.0] - 2021-05-26

## [1.0.2] - 2020-08-04

### Fixed

- Prevent empty resources on other AWS installations with route53 support.

## [1.0.1] - 2020-07-08

### Fixed

- Fix clusterrole template to pass openapi validation.

## [v1.0.0] 2020-06-18

### Added

- Initial release to control plane catalog.

[Unreleased]: https://github.com/giantswarm/route53-manager/compare/v1.4.0...HEAD
[1.4.0]: https://github.com/giantswarm/route53-manager/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/giantswarm/route53-manager/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/giantswarm/route53-manager/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/giantswarm/route53-manager/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/giantswarm/route53-manager/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/giantswarm/route53-manager/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/giantswarm/route53-manager/releases/tag/v1.0.0
