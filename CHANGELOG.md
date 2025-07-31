# Changelog

<!--
    Please refer to https://github.com/truefoundry/KubeElasti/blob/main/CONTRIBUTING.md#Changelog and follow the guidelines before adding a new entry.
-->

## 0.1.15
* Add validation for CRD fields for elasti service by @ramantehlan in https://github.com/truefoundry/elasti/pull/122
* Forward source host to target by @ramantehlan in https://github.com/truefoundry/elasti/pull/123

## 0.1.15-beta (2025-07-28)

### Fixes
* Forward source host to target by @ramantehlan in https://github.com/truefoundry/elasti/pull/159

### Improvements
* Supporting second level cooldown period for prometheus uptime check by @shubhamrai1993 in https://github.com/truefoundry/KubeElasti/pull/125
* Add validation for CRD fields for elasti service by @ramantehlan in https://github.com/truefoundry/elasti/pull/138

### Other
* Add E2E tests via Kuttl by @ramantehlan in https://github.com/truefoundry/KubeElasti/pull/123
* Add Docs for KubeElasti at https://kubeelasti.dev by @ramantehlan in https://github.com/truefoundry/KubeElasti/pull/142
* Bump golang.org/x/oauth2 from 0.21.0 to 0.27.0 in /pkg in https://github.com/truefoundry/KubeElasti/pull/156
* Bump golang.org/x/oauth2 from 0.21.0 to 0.27.0 in /operator in https://github.com/truefoundry/KubeElasti/pull/155
* Bump golang.org/x/oauth2 from 0.21.0 to 0.27.0 in /resolver in https://github.com/truefoundry/KubeElasti/pull/151
* Security Fix: Bump golang.org/x/net from 0.33.0 to 0.38.0 in /pkg in https://github.com/truefoundry/KubeElasti/pull/143

### New Contributors
* @rethil made their first contribution in https://github.com/truefoundry/KubeElasti/pull/154

## 0.1.14
* update workflow to update grype config by @DeeAjayi in https://github.com/truefoundry/KubeElasti/pull/113
* Add support for namespace scoped elasti controller and fixes for cooldown period tracking by @shubhamrai1993 in https://github.com/truefoundry/KubeElasti/pull/115
* Increasing elasti timeout to 10 minutes by @shubhamrai1993 in https://github.com/truefoundry/KubeElasti/pull/116
* corrected the target name being passed by @shubhamrai1993 in https://github.com/truefoundry/KubeElasti/pull/118
* using event recorder for emitting events by @shubhamrai1993 in https://github.com/truefoundry/KubeElasti/pull/117
* dont scale up replicas if the current replicas are greater by @shubhamrai1993 in https://github.com/truefoundry/KubeElasti/pull/119
* fix: port 5000 is used by the systems, using it might be hurdle for u… by @ramantehlan in https://github.com/truefoundry/KubeElasti/pull/120
* dont scale down new service and handle missing prom data by @shubhamrai1993 in https://github.com/truefoundry/KubeElasti/pull/121

All the unreleased changes are listed under `Unreleased` section.

## History

- [Changelog](#changelog)
  - [0.1.15](#0115)
  - [0.1.15-beta (2025-07-28)](#0115-beta-2025-07-28)
    - [Fixes](#fixes)
    - [Improvements](#improvements)
    - [Other](#other)
    - [New Contributors](#new-contributors)
  - [0.1.14](#0114)
  - [History](#history)
  - [Unreleased](#unreleased)

## Unreleased

<!--
    Add new changes here and sort them alphabetically.
Example -
- **General**: Add support for statefulset as a scale target reference ([#10](https://github.com/truefoundry/elasti/pull/10))
-->
