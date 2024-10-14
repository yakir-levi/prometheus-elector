# prometheus-elector

## Acknowledgements

This project is a fork of [prometheus-elector](https://github.com/jlevesy/prometheus-elector) The original project was created by [Jean Levesy](https://github.com/jlevesy). 
All credit for the original work goes to them.

## Changes Made

To ensure that Prometheus Elector works seamlessly in a Prometheus Operator environment,
the following changes have been implemented to enhance functionality:

- Added a new flag --leader-config, which specifies the path to the Prometheus leader configuration file.
- Implemented a mechanism to watch for any changes to the leader-config file, as well as the configuration file generated 
by the Prometheus Operator. 



## Installation

Provide instructions on how to install and run your project.

```bash
# Example installation commands
git clone https://github.com/yourusername/your-repo-name.git
cd your-repo-name
# Additional installation steps
