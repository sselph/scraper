hasher
=======
The initial attempts to port the hashing code to c++. Still needs a ton of cleanup, headers, etc.

Compile
-------
```bash
$ g++ -Wall -W -Werror hasher.cpp -o hasher -lboost_filesystem -lboost_system
```

Usage
-----
```bash
$ hasher [file]...
```
