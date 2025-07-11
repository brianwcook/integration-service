# Integration Service Documentation

## ADR-0033 Architecture (Current)

The Integration Service implements the ADR-0033 architecture with the following active controllers:

### Core Controllers
- [testsubject-controller](testsubject-controller.md) - Orchestrates integration testing for TestSubjects
- [testsubjectconstructor-controller](testsubjectconstructor-controller.md) - Creates TestSubjects from external events
- [scenario-controller](scenario-controller.md) - Manages IntegrationTestScenario resources

### Architecture Documentation
- [ADR-0033 Refactoring Guide](adr-0033-refactoring-guide.md) - Complete architecture overview and migration guide
- [Testing Guide](testing.md) - Testing approaches and best practices

## Legacy Controllers (Deprecated)

The following controllers were part of the legacy snapshot-based architecture and are no longer active:

- ~~snapshot-controller~~ - Replaced by TestSubject/TestSubjectConstructor workflow
- ~~build-pipeline-controller~~ - Functionality absorbed into TestSubjectConstructor
- ~~integration-pipeline-controller~~ - Replaced by TestSubject controller
- ~~component-controller~~ - No longer needed with new architecture
- ~~statusreport-controller~~ - Status tracking moved to TestSubject status

> **Note**: Legacy controller code is retained in `internal/controller/` for reference during the migration period. 
> Consider removing obsolete controllers (snapshot, buildpipeline, component, statusreport) in a future cleanup PR.

## Creating or editing Mermaid diagrams

Mermaid is a JS based diagramming tool that renders markdown style syntax to create/modify diagrams. Mermaid has [native support in Github](https://github.com/github/roadmap/issues/372)

## Editing Diagrams
- Diagrams are stored in the `/docs` folder in the repository. As changes are made to the behaviour of controllers, these changes should also be updated in the diagram. 
- The updated mermaid diagram can be submitted as part of a separate PR or part of the PR that changes the behaviour of the controller. 
- To view diagram in Vscode: right click on file and select 'open preview' after the [plugin](https://marketplace.visualstudio.com/items?itemName=bierner.markdown-mermaid) has been installed (restart required)
- Diagrams can also be created/edited online, live editing is available at [mermaid.live](https://mermaid.live/edit). Copy and paste the diagram code into the live editor and view changes in real time. 

## Resources
- [Information about Mermaid and the types of diagrams available](https://mermaid.js.org/intro/)
- [User guide for beginners](https://mermaid.js.org/intro/n00b-gettingStarted.html)
- Mermaid syntax [cheat sheet](https://jojozhuang.github.io/tutorial/mermaid-cheat-sheet/)
- Vscode extension [Markdown Preview Mermaid Support](https://marketplace.visualstudio.com/items?itemName=bierner.markdown-mermaid)

