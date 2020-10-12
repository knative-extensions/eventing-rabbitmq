# Rigging

Write end-to-end tests for Kubernetes leveraging lightly templated yaml files. 

## Setup

Rigging depends on [ko](https://github.com/google/ko) to run and be configured in the context of the go test run.

After you have `ko` working and a target Kubernetes cluster with `kubectl`,
testing should be as simple as running:
 
 ```shell
go test ./example/... -tags e2e -v
```

Or to just run `TestFoo`:

```shell
go test ./example/... -run Foo -tags e2e -v -count=1
```


# Rigging Architecture

> Note: Rigging assumes the cluster is ready for testing. You can use rigging to setup dependent resources, but this should be limited
to resources that are namespaced. 

Rigging will create a namespace to be used for the test. All dependent namespace scoped resources that the test depends on
should be included in the rig. The namespace used in Rigging will be deleted after the test has finished. This allows
Rigging to run many tests in parallel assuming the test author understands any global cluster concerns.

## Phases of a Rigging Test

1. Tests register their interest in an image by `RegisterPackage` in their init method.
1. Test calls `rigging.NewInstall` and receives a `rig` instance to be used for further testing and cleanup.

    Internally, rigging will:
    1. Apply all given options to the test rig.
    1. Create a new generated namespace.
    1. Produce images (using `ko`).
    1. Apply config to the given set of yaml config files.
    1. Apply template processed config files to the kubernetes cluster.
1. Test performs the test implementation.
1. Test collects results and determine PASS/FAIL. 
1. Test calls `rig.Uninstall()` to delete all resources and the test namespace.

    Internally, rigging will:
    1. Delete all resources created by the rig.
    1. Delete the generated namespace, if created by the rig.



