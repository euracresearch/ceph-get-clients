# ceph-get-clients

### WARNING: We will not guarantee that the output of this tool is correct. Use it on your own risk.

Ceph-get-clients returns the current connected clients

Usage:

```
ceph-get-clients -user cephssh [-port 22 -feature 0x200000] mon1 mon2 mon3
```

Ceph-get-clients will connect to the given Ceph monitor servers using SSH and
retrieve all currently connected clients using `ceph daemon mon.<hostname> sessions`. 
It will parse the output and merge it for all the given monitors,
duplicated clients will be removed. For each client a reverse DNS lookup will
be done. The output will be printed to Stdout using CSV format. It is
possible to check if a client supports a give feature by passing the feature
hex value as a parameter using the -feature flag.

Example:

```
ceph-get-client -user cephadm -feature 0x200000 mon1 mon2 mon3
IP,feature,release,fqdn,0x200000
10.7.3.67,0x3ffddff8eea4fffb,luminous,clienta.fqdn.tld.,true
10.7.3.65,0x3ffddff8eea4fffb,luminous,webserver.fqdn.tld.,true
10.7.3.64,0x7010fb86aa42ada,jewel,,true
10.7.3.70,0x1ffddff8eea4fffb,luminous,usera.fqdn.tld.,true
```