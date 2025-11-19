#!/bin/bash

# Debug script for testing MediaRemote helper

echo "Building helper..."
cd helpers/nowplaying
make clean && make
cd ../..

echo ""
echo "Testing helper with debug command..."
echo "Please make sure audio is playing (YouTube, Apple Music, Spotify, etc.)"
echo ""

./helpers/nowplaying/nowplaying debug

echo ""
echo "If you see '(empty - no data returned)' above, it means:"
echo "1. Your browser/app isn't sending Now Playing info to macOS"
echo "2. You may need to enable media controls in your browser"
echo "3. Try Safari or Chrome - they usually work better"
echo ""
echo "If you see keys and values, we can use those to fix the detection!"
