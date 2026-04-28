# Juliet

![Architecture diagram showing queue workers processing markdown jobs](architecture.png)

## Overview

Stable fact: juliet retries failed jobs three times.

## Workers

Workers acknowledge jobs only after writing the compressed output.
