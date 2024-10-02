@echo off
set location=your-city-name-country-name

set outputFileSubnet1=%location%-starlink-backbone-traceroute-149.19.txt
set outputFileSubnet2=%location%-starlink-backbone-traceroute-206.224.txt

echo > %outputFileSubnet1%
echo > %outputFileSubnet2%

for /L %%i in (108,1,109) do (
    for /L %%j in (0,1,255) do (
        echo Tracert to 149.19.%%i.%%j
        tracert -h 18 -w 1000 149.19.%%i.%%j >> %outputFileSubnet1%
    )
)

for /L %%i in (64,1,70) do (
    for /L %%j in (0,1,255) do (
        echo Tracert to 206.224.%%i.%%j
        tracert -h 18 -w 1000 206.224.%%i.%%j >> %outputFileSubnet2%
    )
)

echo Starlink backbone traceroute completed. Results saved to %outputFileSubnet1% and %outputFileSubnet2%

