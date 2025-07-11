---
title: Integrations with HPA
---


# Integration with HPA
KubeElasti works seamlessly with the Horizontal Pod Autoscaler (HPA) and handles scaling to zero on its own. Since KubeElasti manages the scale-to-zero functionality, you can configure HPA to handle scaling based on metrics for any number of replicas **greater than zero**, while KubeElasti takes care of scaling to/from zero.

A setup is explained in the [getting started](getting-started.md) guide.
