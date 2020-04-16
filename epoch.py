#!/usr/bin/python

import sys
import time

print("Enter your list of epoch times. Press Ctrl+d when done\n")

convTime = sys.stdin.readlines()

for eachTime in convTime:
    newTime = time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(int(eachTime)))
    print(eachTime.strip('\n')+" = "+newTime)
