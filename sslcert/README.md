# These certificates are only for development

To generate a certificate for development run the following in this directory (./sslcert)
```
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout tls.key -out tls.crt -config req.conf -extensions 'v3_req'
```

OR

```
./../setup.sh
``