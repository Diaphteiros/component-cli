## component-cli component-archive signatures verify x509

fetch the component descriptor from an oci registry and verify its integrity based on a x509 certificate chain and a RSASSA-PKCS1-V1_5 signature

```
component-cli component-archive signatures verify x509 BASE_URL COMPONENT_NAME VERSION [flags]
```

### Options

```
      --allow-plain-http               allows the fallback to http if the oci registry does not support https
      --cc-config string               path to the local concourse config file
      --cert string                    path to a file containing the certificate file in PEM format
  -h, --help                           help for x509
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --intermediate-ca-certs string   [OPTIONAL] path to a file containing the concatenation of any intermediate ca certificates in PEM format
      --registry-config string         path to the dockerconfig.json with the oci registry authentication information
      --root-ca-cert string            [OPTIONAL] path to a file containing the root ca certificate in PEM format. if empty, the system root ca certificate pool is used
      --signature-name string          name of the signature to verify
      --signing-cert string            (deprecated: use --cert instead) path to a file containing the signing certificate file in PEM format
```

### Options inherited from parent commands

```
      --cli                  logger runs as cli logger. enables cli logging
      --dev                  enable development logging which result in console encoding, enabled stacktrace and enabled caller
      --disable-caller       disable the caller of logs (default true)
      --disable-stacktrace   disable the stacktrace of error logs (default true)
      --disable-timestamp    disable timestamp output (default true)
  -v, --verbosity int        number for the log level verbosity (default 1)
```

### SEE ALSO

* [component-cli component-archive signatures verify](component-cli_component-archive_signatures_verify.md)	 - command to verify the signature of a component descriptor

