---
name: Starlink Backbone Traceroute Report
about: Traceroute/tracert report from Starlink user side to backbone routers
title: "[traceroute] [CITY]-[COUNTRY]"
labels: traceroute
assignees: clarkzjw

---

## Where is your Starlink dish? (You only need to provide the city name and country name)

**City**:
**Country**:

## What is the hostname of your Starlink IP address? (You do **NOT** need to provide your IP address publicly)

**Hostname**: 

Howto:

1. You can visit https://ipconfig.io/json or https://www.whatsmyip.org/ and only copy the string for `hostname` or `Your Hostname:`.

2. You can also run `nslookup $(curl -s -4 ipconfig.io)` or `nslookup $(curl -s -4 ipconfig.io)` and only copy the string after `name =`.

3. In either case, your hostname should be in the format of `customer.<PoP>.pop.starlinkisp.net.`, e.g., `customer.tkyojpn1.pop.starlinkisp.net.`

## Traceroute/tracert result

Please attach below your traceroute results by uploading the two files created by running the scripts at https://github.com/clarkzjw/starlink-lens/tree/master/backbone-map depending on your operating system.

**Result**:
