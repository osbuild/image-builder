# Legacy bootc-image-builder upload command

This contains the "legacy" upload command that was part of the
original bootc-image-builder source. We need to keep it around and put
it into the "bootc-image-builder" container to not break existing
users.

But it should not be used in the image-builder container or RPM. One
big issue is that we do not have enough data about the image to know
e.g. if it needs to be uploaded as a BIOS or hybrid image etc.
