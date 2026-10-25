# Host-driven version

Implemented: passive firmware, a machine-readable USB protocol and one Windows EXE with
service, install, uninstall, self-update and user CLI modes. The architecture separates the CEC
device, frame sequences, configuration and the Windows adapters, so the core logic is tested
on a Mac.

The remaining physical checks are listed in [VALIDATION.md](VALIDATION.md).
