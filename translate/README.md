# What is this?

I was bored and looking at a LinkedIn post where someone was saying they
needed Putty or SecureCRT to have a plugin that changed IP addresses
to hostnames for a Cisco BGP command.

So I wrote a SSH client emulating VT100 that spawns an SSH shell
(does NOT call the ssh command) and logs into a device. When you issue
a command through the shell, it looks for an IPv4 address in the
stream using a statemachine. If it finds one, it does a DNS lookup
and replaces the IP with what it found. If it find multiple names, it
does just the first one. If it can't find one, it does nothing.

# Notes

 * DNS lookup happens on the local machine, not the remote.
 * This is line oriented based on '\n'. Beware of the '\r'.
 * This is not a full functioning terminal, so I wouldn't try to use vim/emacs via this.

# What is this good for?

Could use this as a basis for a tool that does this for single commands you want to
run on a few devices.  Simple change the Shell() to Run(). Actually the translator
will work on files too.

Or you could just log into a device and run your commands using this.

This was really something I thought might help the LinkedIn poster and a way
to keep my SSH library skills up. Don't have much use for them currently and I needed
a break from the other stuff I was working on.

# Example

This simply logs into a host, I cat a file with an IP(8.8.8.8*) and a CIDR address(8.8.8.8/32)
and watch the output get translated. Any IPv4 address in the output will get translated from
any command.
```bash
./translate user@hostname:22

Welcome to Ubuntu 18.04.2 LTS (GNU/Linux)

 * Documentation:  https://help.ubuntu.com
 * Management:     https://landscape.canonical.com
 * Support:        https://ubuntu.com/advantage

prompt>cat file.txt
dns.google.
8.8.8.8/32
prompt>
```

The file file.txt actually contains:
```
8.8.8.8
8.8.8.8/32
```
