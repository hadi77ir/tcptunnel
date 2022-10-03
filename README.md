# TCPTunnel
Forwards every received TCP socket through given proxy or directly to the given remote host.

## Usage
```
tcptunnel -listen :80 -proxy socks5://127.0.0.1:1080/ -target 10.10.34.35:80
```

## License
The Apache License, Version 2.0 - see LICENSE for more details.

## Credits
Contains some code from [gsocks5](https://github.com/buraksezer/gsocks5), used under Apache License 2.0.
Copyright (C) 2017 Burak Sezer ([buraksezer](https://github.com/buraksezer)).