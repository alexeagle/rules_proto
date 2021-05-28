# `rules_proto`

Bazel starlark rules for building protocol buffers +/- gRPC :sparkles:.

<table border="0"><tr>
<td><img src="https://bazel.build/images/bazel-icon.svg" height="180"/></td>
<td><img src="https://github.com/pubref/rules_protobuf/blob/master/images/wtfcat.png" height="180"/></td>
<td><img src="https://avatars2.githubusercontent.com/u/7802525?v=4&s=400" height="180"/></td>
</tr><tr>
<td>Bazel</td>
<td>rules_proto</td>
<td>gRPC</td>
</tr></table>

`stackb/rules_proto` provides a gazelle extension that generates `proto_library`-derived BUILD rules for your bazel project.

Here's what the developer workflow looks like:

```bash
# create a directory for protos
$ mkdir proto/

# write a proto file
$ echo 'syntax = "proto3"; message Foo{}' > proto/foo.proto

# initialize a build file
$ touch proto/BUILD.bazel

# declare a new language configuration called "py" and enable it.  "py" is just a string, choose whatever name you like.
$ echo '# gazelle:proto_language py enable true' >> proto/BUILD.bazel
# associate the builtin "python" plugin with the "py" language configuration.
# This registers an implementation function that teaches gazelle what files will be
# generated by the --python_out option. This "builtin:python" implementation comes 
# preinstalled; you can register your own custom implementation using a golang init function like `func init() { protoc.MustRegisterPlugin(&myCustomPlugin)}) }`.
$ echo '# gazelle:proto_language py plugin builtin:python' >> proto/BUILD.bazel
# associate a rule "proto_compile" with the "py" language configuration.  This teaches
# gazelle how to generate that kind of rule.  "proto_compile" comes preinstalled but you 
# can register your own using a golang init() function.
$ echo '# gazelle:proto_language py rule proto_compile' >> proto/BUILD.bazel

# run gazelle, update the build files.  proto/BUILD.bazel should now have a new "proto_library"
# rule and a new "proto_compile" rule.
$ bazel run //:gazelle  

# build the "proto_compile" rule (this executes the protoc action)
$ bazel build //proto:foo_py_compile
```

# Getting Started

You can use `rules_proto` without the gazelle extension, but this document describes getting that setup in your project.

## Step 1: ensure rules_go and bazel-gazelle WORKSPACE dependencies

If you don't already have these dependencies in your `WORKSPACE`

OK: you built the depsgen thing.  Now what?  How does it help you add tests or
improve docs?  Sorta wanted to have the full end-to-end workflow of testing and
docs in place before adding more plugins and increasing that surface area.

What will be the pages in the docsite?  Examples?  Might be a good time to start
writing the docs and see how they look.  At the end of the day you need
reproducible examples that run as example_bazel_tests.

- split examples for each language
- add docs foreach example
- protoc-gen-go
- protoc-gen-go-grpc
- protoc-gen-go