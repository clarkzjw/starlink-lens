build:
	go install -ldflags "-s -w" ./cmd/lens

install:
	mkdir -p ${DESTDIR}/usr/bin
	cp ${HOME}/go/bin/* ${DESTDIR}/usr/bin

ppa:
	dh_make -p lens_0.0.0 --createorig
	debuild -S -k$DEBSIGN_KEYID
	cd ..
	dput ppa:clarkzjw/lens lens_0.0.0-14_source.changes
