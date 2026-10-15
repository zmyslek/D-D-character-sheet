# Standards and engineering practices

Identifiers and editions were checked on 2026-10-15 against public catalogue
pages and summaries (the standard texts themselves are paid documents and were
not consulted). This project does **not** claim conformance to any of them; the
table shows which practices were borrowed and where.

| Standard | Title (short) | Edition used | Applied in this project | Not applied |
|---|---|---|---|---|
| ISO/IEC 25010 | Systems and software Quality Requirements and Evaluation (SQuaRE) – Product quality model | 2023 (2nd edition) | Quality characteristics used to structure NFRs and the maintainability assessment ([QUALITY_AND_TESTING.md](QUALITY_AND_TESTING.md), [REQUIREMENTS.md](REQUIREMENTS.md)) | No formal evaluation per ISO/IEC 25040; the measures come from our own tools, not ISO/IEC 25023 |
| ISO/IEC/IEEE 29148 | Life cycle processes – Requirements engineering | 2018 | Requirements have IDs, actor, preconditions, acceptance criteria and a verification method; traceability to tests ([REQUIREMENTS.md](REQUIREMENTS.md)) | No full SRS document structure; no stakeholder analysis |
| ISO/IEC/IEEE 42010 | Software, systems and enterprise – Architecture description | 2022 (2nd edition) | Architecture is described through views for different concerns: context, containers, components, domain, data, deployment ([DIAGRAMS.md](DIAGRAMS.md), [ARCHITECTURE.md](ARCHITECTURE.md)); decisions with rationale (DDD.md §6) | No formal viewpoint definitions or stakeholder/concern tables |
| ISO/IEC/IEEE 29119 (parts 1–3) | Software testing – concepts, processes, documentation | 29119-1:2022 and later parts | Test levels and techniques documented; test cases with ID, objective, preconditions, steps, expected and actual result ([MANUAL_TEST_PLAN.md](MANUAL_TEST_PLAN.md)); traceability matrix | No separate test policy, organisational test process or completion report |
| ISO/IEC/IEEE 12207 | Software life cycle processes | 2017 (a newer edition may exist; check before citing) | Implementation, integration, verification and maintenance activities are visible (refactoring in small verified steps, automated tests, static analysis) | No full process tailoring |

Related tools/conventions that are **not** standards: the C4 model (diagram
notation by Simon Brown), Mermaid (diagram syntax), SonarQube "Sonar way"
quality gate, SIG-style risk bands in `tools/metrics`.

## References

* ISO/IEC 25010:2023 – catalogue summary: <https://standards.iteh.ai/catalog/standards/iso/8c5bd63e-9e67-4b55-90e3-8cb2b10a2030/iso-iec-25010-2023>
* arc42 quality model on ISO 25010: <https://quality.arc42.org/standards/iso-25010>
* SonarSource, "ISO/IEC 25010 explained": <https://www.sonarsource.com/resources/library/iso-iec-25010-explained/>
* ISO/IEC/IEEE 29148-2018 at IEEE SA: <https://standards.ieee.org/standard/29148-2018.html>
* ISO/IEC/IEEE 42010:2022: <https://webstore.ansi.org/standards/iso/isoiecieee420102022>
* ISO/IEC/IEEE 29119 overview: <https://en.wikipedia.org/wiki/ISO/IEC_29119>
* SRD 5.1 (the domain rules): `Instructions/SRD_CC_v5.1.pdf`
