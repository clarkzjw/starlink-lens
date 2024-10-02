# Starlink Backbone Map

## How to participate

If you have (access to) a Starlink dish, you can help the community identifying the Starlink backbone map topology by running the backbone traceroute scripts.

### Linux

Ideally, you should have access to a Linux like environment, either it is a Linux machine, or a WSL (Windows Subsystem for Linux) environment.

You may also want to install the `traceroute` package instead of the default `inetutils-traceroute` package on some Linux distributions. Check the version installed by `apt list --installed | grep traceroute` on Debian-based Linux distributions.

[traceroute-bb.sh](./script/traceroute-bb.sh)

### Windows

If you only have access to a Windows machine and you cannot install WSL or a Linux virtual machine, you can run a simplified version of the script in PowerShell. 

[traceroute-bb.bat](./script/traceroute-bb.bat)

In either case, you can download the corresponding script and run it in your environment. The running time of the traceroute script is approximately a few hours.

Submit your `traceroute`/`tracert` result by creating an issue [here](https://github.com/clarkzjw/starlink-lens/issues/new?assignees=clarkzjw&labels=traceroute&projects=&template=starlink-backbone-traceroute-report.md&title=%5Btraceroute%5D+%5BCITY%5D-%5BCOUNTRY%5D).

## Reference:

Our previous work [1] partially contributes to the creation of the map of [Unofficial Starlink Global Gateways & PoPs](https://tinyurl.com/starlinkmap).

[1]. J. Pan, J. Zhao and L. Cai, "[Measuring a Low-Earth-Orbit Satellite Network](https://ieeexplore.ieee.org/document/10294034)", IEEE PIMRC'23, doi: 10.1109/PIMRC56721.2023.10294034, [[arXiv]](https://doi.org/10.48550/arXiv.2307.06863)

[2]. https://oac.uvic.ca/starlink

[3]. [r/StarlinkEngineering: run_a_few_scripts_behind_your_starlink_dish](https://www.reddit.com/r/StarlinkEngineering/comments/17vche2/run_a_few_scripts_behind_your_starlink_dish/)
