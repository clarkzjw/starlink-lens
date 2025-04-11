build:
	go install -ldflags "-s -w" ./cmd/lens

install:
	mkdir -p ${DESTDIR}/usr/bin
	cp ${HOME}/go/bin/* ${DESTDIR}/usr/bin
