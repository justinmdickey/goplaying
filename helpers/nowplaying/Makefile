.PHONY: all clean

all: libnowplaying.dylib

libnowplaying.dylib: NowPlaying.swift
	swiftc -O -parse-as-library -emit-library NowPlaying.swift -o libnowplaying.dylib

clean:
	rm -f libnowplaying.dylib nowplaying
