# Standard library

The Unobin standard library provides resources and actions for common
tasks such as operating on files, zip archives, processes, and HTTP requests.

```
factory: {
  description: 'Writes an app config file.'

  inputs: {
    config-path: { type: string }
    app-name:    { type: string }
  }

  imports: { std: 'github.com/cloudboss/unobin-library-std' }

  resources: {
    config: std.fs-file {
      path: input.config-path
      content: @core.to-json({ app: input.app-name })
      create-directory: true
    }
  }

  outputs: {
    config-sha256: { value: resource.config.sha256 }
  }
}
```

Add the library to the dependency project before compiling the factory:

```
unobin deps get github.com/cloudboss/unobin-library-std@v0.2.1
```

Create a zip archive from a directory:

```
resources: {
  package: std.archive-zipfile {
    path: './build/app.zip'
    source-dir: './app'
    create-directory: true
    excludes: ['**/.git/**']
  }
}
```

## Configuration

The standard library has no library configuration.

## Reference

The generated reference lists every resource and action kind, its inputs,
outputs, defaults, and sensitive fields.

- [Resources](reference/resources/index.md)
- [Actions](reference/actions/index.md)
