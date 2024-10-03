# Overview

A simple poc that uses the buildah golang libraries, to build and push multi-arch images

For the poc I used the redhat-oprerator-index with a docker file to rebuild catalogs for each
architecture, get a link to the manifest list and finally push the "rebuild-catalog" to the remote
registry

