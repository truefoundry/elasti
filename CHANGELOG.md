# Changelog

<!--
    Please refer to https://github.com/truefoundry/elasti/blob/main/CONTRIBUTING.md#Changelog and follow the guidelines before adding a new entry.
-->

## 0.1.15
* Add validation for CRD fields for elasti service by @ramantehlan in https://github.com/truefoundry/elasti/pull/122

## 0.1.14
* update workflow to update grype config by @DeeAjayi in https://github.com/truefoundry/elasti/pull/113
* Add support for namespace scoped elasti controller and fixes for cooldown period tracking by @shubhamrai1993 in https://github.com/truefoundry/elasti/pull/115
* Increasing elasti timeout to 10 minutes by @shubhamrai1993 in https://github.com/truefoundry/elasti/pull/116
* corrected the target name being passed by @shubhamrai1993 in https://github.com/truefoundry/elasti/pull/118
* using event recorder for emitting events by @shubhamrai1993 in https://github.com/truefoundry/elasti/pull/117
* dont scale up replicas if the current replicas are greater by @shubhamrai1993 in https://github.com/truefoundry/elasti/pull/119
* fix: port 5000 is used by the systems, using it might be hurdle for u… by @ramantehlan in https://github.com/truefoundry/elasti/pull/120
* dont scale down new service and handle missing prom data by @shubhamrai1993 in https://github.com/truefoundry/elasti/pull/121

All the unreleased changes are listed under `Unreleased` section.

## History

- [Changelog](#changelog)
  - [0.1.15](#0115)
  - [0.1.14](#0114)
  - [History](#history)
  - [Unreleased](#unreleased)

## Unreleased

<!--
    Add new changes here and sort them alphabetically.
Example -
- **General**: Add support for statefulset as a scale target reference ([#10](https://github.com/truefoundry/elasti/pull/10))
-->
