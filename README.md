# nginx2es 

Collect nginx logs to ES

# socket: too many open files

```
vim /etc/security/limits.conf

# add for ES
# {
* soft nofile 65536
* hard nofile 131072
* soft nproc 4096
* hard nproc 4096
#}
```

