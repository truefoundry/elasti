# KubeElasti Release Process

This document outlines the release process for KubeElasti, covering both beta and stable releases.

## Release Workflow Overview

KubeElasti has 3 release types:
1. **Stable Release**: Manually triggered via Github releases
2. **Beta Release**: Manually triggered via Github releases
3. **Beta Releases (Legacy)**: Automated from the `main` branch


## 1. Stable Release

Stable releases require manual preparation and are triggered by creating a GitHub release.

### Preparation Steps

1. Update version information:
   
   - Update `charts/elasti/Chart.yaml` with the new version number:
     ```yaml
     version: X.Y.Z
     appVersion: "X.Y.Z"
     ```
   
   - Update `charts/elasti/values.yaml` to reference the specific commit SHA:
     ```yaml
     elastiController:
       manager:
         image:
           tag: vX.Y.Z
     elastiResolver:
       proxy:
         image:
           tag: vX.Y.Z
     ```

2. Create a pull request with these changes
3. Review and merge the PR to `main`

### Release Steps

1. Create a new GitHub release:
   - Tag format: `vX.Y.Z`
   - Title: `KubeElasti vX.Y.Z`
   - Include release notes detailing changes
        ```markdown
        We are happy to release KubeElasti vX.Y.Z 🎉

        Here are some highlights of this release:
        - Highlight 1
        - Highlight 2

        Here are the breaking changes of this release:
        - Breaking Change 1
        - Breaking Change 2

        Learn how to deploy KubeElasti by reading [our documentation](https://kubeelasti.dev).

        # New
        - <Where the change was done>: <What was added>
        - <Resolved>: ...
        - <General>: ...

        ## Experimental
        - <Where the change was done>: <What was added>

        # Improvements
        - <Where the change was done>: <What was improved>

        # Fixes
        - <Where the change was done>: <What was fixed>

        # Breaking Changes
        - <Where the change was done>: <What was changed>

        # Other
        - <Where the change was done>: <What was changed>

        # New Contributors
        - @rethil made their first contribution in #154
        etc...
        ```
    - Feel free to add more sections as needed, or remove what is not needed.


2. The `.github/workflows/release.yaml` workflow is triggered:
   - Helm chart is packaged
   - Chart is pushed to the JFrog Artifactory Helm repository


## 2. Beta Release

Same steps as stable, we just replace it with `vX.Y.Z-beta`.


## 3.  Beta(Legacy) Release

Beta(Legacy) releases are automatically generated when code is merged to the `main` branch.

### Workflow

1. Code is merged to the `main` branch
2. The `.github/workflows/build-n-publish.yml` workflow is triggered:
   - Docker images are built for both operator and resolver components
   - Images are tagged with the commit SHA
   - Images are pushed to JFrog Artifactory
   - The `helm-main` branch is updated with the latest SHA in `values.yaml`

### Using Beta Releases

Beta releases can be used for testing by referencing:
- The specific commit SHA from the Docker images
- The Helm chart from the `helm-main` branch


## Version Numbering

KubeElasti follows semantic versioning (SemVer):
- **X**: Major version for incompatible API changes
- **Y**: Minor version for new functionality in a backward-compatible manner
- **Z**: Patch version for backward-compatible bug fixes


## Rollback Procedure

If issues are discovered in a release:
1. For critical issues, create a hotfix release
2. For stable releases.
   1. Prepare a new patch release with fixes.
   2. Mark the old release as deprecated or bad.


