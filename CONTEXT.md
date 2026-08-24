# Phite

Phite is an open-source local development environment for PHP projects. It exists to make a project ready for development without requiring the developer to assemble a local server stack.

## Language

**PHP Project**:
A local source directory containing a PHP application that a developer wants to run and change.
_Avoid_: Site, codebase, repository

**Development Environment**:
The runnable local state of a PHP Project, including its application endpoint and supporting capabilities.
_Avoid_: Stack, setup

**Developer**:
The person using Phite to work on a PHP Project, primarily independently or in a small team.
_Avoid_: User, operator

**Supported Project**:
A PHP Project whose web requests enter through a front controller. Laravel is an initial compatibility fixture, not a separate product category.
_Avoid_: Laravel Project, framework integration

**Managed Runtime**:
The PHP execution environment supplied and controlled by Phite rather than installed globally by the Developer.
_Avoid_: System PHP, local PHP

**Project Database**:
A database whose lifecycle and connection information belong to one PHP Project rather than being shared across the machine.
_Avoid_: Database server, global database

**Live Reload**:
A full browser refresh triggered by a relevant PHP Project file change. Frontend module replacement remains the responsibility of the Project's frontend tooling.
_Avoid_: HMR, hot reload

**Project Configuration**:
Version-controlled declarations that override Phite's detected Development Environment for one PHP Project.
_Avoid_: Settings, preferences

**Local State**:
Generated, machine-local data that Phite owns for one PHP Project and that is excluded from version control.
_Avoid_: Project Configuration, source files

**Tier 1 Platform**:
An operating system and architecture combination on which every Phite release is integration-tested and supported.
_Avoid_: Compatible platform

**Experimental Platform**:
An operating system and architecture combination for which Phite publishes an artifact without guaranteeing integration-test coverage.
_Avoid_: Tier 1 Platform

**Development Session**:
The supervised lifetime during which one PHP Project is locally runnable through Phite.
_Avoid_: Server, process

**Local Endpoint**:
The HTTP address through which a Developer accesses a PHP Project during a Development Session.
_Avoid_: Production URL, domain

**Required Capability**:
A capability whose failure makes a Development Session unusable and causes startup to stop.
_Avoid_: Core service

**Optional Capability**:
A capability whose failure is reported without ending an otherwise usable Development Session.
_Avoid_: Add-on, plugin

**Runtime Identity**:
The complete identity of a Managed Runtime, including its FrankenPHP and PHP versions, platform, extension set, and artifact checksum.
_Avoid_: PHP version, FrankenPHP version

**Runtime Manifest**:
The trusted record that maps a Runtime Identity to an immutable runtime artifact and its verification data.
_Avoid_: Download list, release metadata

**Runtime Cache**:
The Developer-scoped store of verified Managed Runtime artifacts shared by PHP Projects on one machine.
_Avoid_: Local State, vendor directory

**Runtime Compatibility**:
The condition in which a Managed Runtime satisfies a PHP Project's declared PHP and extension requirements.
_Avoid_: Framework support, platform support

**Database Contract**:
The Phite-owned environment variables through which a PHP Project may discover its Project Database without Project file mutation.
_Avoid_: Database configuration, framework adapter

**Document Root**:
The directory from which a Development Session serves public files and locates its Entrypoint.
_Avoid_: PHP Project root, working directory

**Entrypoint**:
The front-controller PHP file that receives non-static HTTP requests for a Supported Project.
_Avoid_: Bootstrap, router

**Runtime Command**:
A Project-scoped PHP or Composer command executed through the Managed Runtime.
_Avoid_: System command, shell command

**Active Session**:
The single Development Session currently owned by a PHP Project. Different PHP Projects may each have an Active Session.
_Avoid_: Process, instance

**Phite CLI**:
The public product identity of the local development environment and its command-line interface.
_Avoid_: Phite Framework, PHP Vite

**Configuration Schema**:
The versioned contract that defines valid Project Configuration and how compatibility is evaluated.
_Avoid_: YAML format, config version

**Third-party Notice Bundle**:
The artifact-specific licenses, acknowledgements, and notices distributed with a Managed Runtime and Composer.
_Avoid_: Phite license, generic notice
