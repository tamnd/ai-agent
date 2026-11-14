<!-- Generated from the code of package node by go test ./schema -update. Do not edit; change the code. -->

# Edges, version 1

Edges are typed fields of the front matter or the card (spec 04.7). The build derives the reverse edges, and the audit rules E01 to E14 check them (15.3). A neighbour node of algorithm, math2 or programming is named through a bridge card (5.9).

| Edge | From | To | Meaning | Acyclic |
|---|---|---|---|---|
| `prereqs` | any | any | read the target first; it forms the reading DAG (5.7) | yes |
| `uses` | method, architecture, system, model | method, architecture, concept, algorithm bridge | it calls, contains or depends on the target |  |
| `components` | architecture | architecture, method | the blocks it is made of |  |
| `variant_of` | method, architecture, metric, benchmark | same kind | it changes the target in a named way | yes |
| `related_to` | any | any | a sibling or a close idea, with a one-line note on the difference |  |
| `generalizes` | concept, method, task | same kind | the target is a special case | yes |
| `applies_to` | method | architecture, task | the method is used with the target |  |
| `solves` | method, architecture, model, system | task | it is used for the task |  |
| `evaluated_on` | model, method | benchmark version | it has results there (derived from result cards) |  |
| `measured_by` | task, benchmark | metric | the metric of the task or the benchmark |  |
| `trained_on` | model | dataset | with the share, when known |  |
| `architecture` | model | architecture | the architecture of the model |  |
| `fine_tuned_from` | model | model | the lineage of weights | yes |
| `distilled_from` | model | model | the lineage of weights |  |
| `merged_from` | model | model | the lineage of weights |  |
| `member_of` | model | model family | the family |  |
| `developed_by` | model, system, software, hardware, benchmark, dataset | organization, person | who made it |  |
| `introduced_by` | any | work | the work that introduced it |  |
| `cites` | work | work | from OpenAlex or Semantic Scholar, only between works that have cards |  |
| `proves` | theorem | concept, method, architecture | the theorem is about the target |  |
| `explains` | concept, theorem | finding | a proposed explanation, with its status in claims.yaml |  |
| `evidence_for` | finding, work, result, experiment | finding, open-problem, claim | evidence for the target, with the claim id (07.3) |  |
| `evidence_against` | finding, work, result, experiment | finding, open-problem, claim | evidence against the target, with the claim id (07.3) |  |
| `instance_of` | incident | risk | the incident is a case of the risk |  |
| `mitigates` | method, policy, standard | risk | it reduces the risk, with the evidence label |  |
| `regulates` | policy, provision | system kind, risk, practice | the law covers it |  |
| `implements_policy` | standard | policy, provision | a harmonized standard or a code of practice for the provision |  |
| `amends` | policy | policy | the amendment |  |
| `responds_to` | argument, work | argument | a reply to the target |  |
| `succeeded_by` | model, hardware, software, benchmark | same kind | the next version | yes |
| `people` | any | person | who found, named, built or improved it |  |
| `orgs` | any | organization | who found, named, built or improved it |  |
| `events` | any | event | it took part in the event |  |
| `exercises` | any | exercise | practice for the target |  |
| `same_as` | any | external ids | Wikidata, Wikipedia, arXiv, DOI, OpenAlex, Semantic Scholar, DBLP, Epoch, Hub, AIID, ORCID |  |
| `see_also` | any | any | a related page, at most 8 |  |

No cycle is allowed in `prereqs`, `variant_of`, `generalizes`, `fine_tuned_from` or `succeeded_by`. The targets of `solves` are tasks, of `evaluated_on` benchmark versions and of `architecture` architectures, and each core node has a `prereqs` path from a foundation node (5.7).
