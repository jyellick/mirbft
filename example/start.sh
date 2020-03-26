#!/bin/bash

for ii in $(seq 0 3) ; do
	./example cryptogen/config${ii}.yaml &> ${ii}.log &
done
