/-
AlgoExtract (spec 07.12). `aik lean build` runs this file with `lake env lean` after `lake build`. It writes one JSON line for each declaration in the namespace Algo to the file that ALGO_DECLS names.
-/
import Algo
import Lean.Util.CollectAxioms

open Lean Elab Command

/-- The kind of a constant, as decls.jsonl names it. -/
def algoKind (env : Environment) (n : Name) : ConstantInfo → String
  | .thmInfo _ => "theorem"
  | .defnInfo _ => if Meta.isInstanceCore env n then "instance" else "def"
  | .inductInfo _ => if isStructure env n then "structure" else "inductive"
  | .axiomInfo _ => "axiom"
  | .opaqueInfo _ => "opaque"
  | _ => "other"

/-- A name that Lean makes for its own use, such as an equation lemma or a matcher. -/
def algoAux (env : Environment) (n : Name) : Bool :=
  n.isInternalDetail || isAuxRecursor env n || isNoConfusion env n || Meta.isMatcherCore env n ||
    match n with
    | .str _ s => s.startsWith "eq_" || s.startsWith "match_" || s.startsWith "proof_" ||
        s == "induct" || s == "mutual_induct" || s == "fun_cases" || s == "inj" || s == "injEq" ||
        s == "sizeOf_spec" || s == "rec" || s == "recOn" || s == "casesOn" || s == "below" || s == "brecOn"
    | _ => false

#eval show CommandElabM Unit from do
  let env ← getEnv
  let some out ← IO.getEnv "ALGO_DECLS" | throwError "ALGO_DECLS is not set"
  let mut names : Array Name := #[]
  for (n, ci) in env.constants.map₁.toList do
    if n.getRoot == `Algo && !algoAux env n && !ci.isCtor && !(ci matches .recInfo _) then
      names := names.push n
  names := names.qsort (fun a b => a.toString < b.toString)
  let mut lines : Array String := #[]
  for n in names do
    let some ci := env.find? n | continue
    let axs ← liftCoreM (collectAxioms n)
    let ty ← liftTermElabM do
      let f ← Meta.ppExpr ci.type
      return toString f
    let doc ← findDocString? env n
    let ranges ← findDeclarationRanges? n
    let mod := match env.getModuleIdxFor? n with
      | some i => (env.header.moduleNames[i.toNat]!).toString
      | none => ""
    let deps := ci.getUsedConstantsAsSet.toList.filter (fun d =>
      (d.getRoot == `Algo || d.getRoot == `Cslib) && !algoAux env d) |>.map (·.toString)
    let deps := deps.toArray.qsort (· < ·)
    let axs := (axs.map (·.toString)).qsort (· < ·)
    let pos : Json := match ranges with
      | some r => Json.mkObj [("line", r.range.pos.line), ("end_line", r.range.endPos.line)]
      | none => Json.null
    let kind := algoKind env n ci
    let j := Json.mkObj [
      ("decl", n.toString),
      ("kind", kind),
      ("module", mod),
      ("type", ty),
      ("type_hash", toString (hash ci.type)),
      ("axioms", toJson axs),
      ("deps", toJson deps),
      ("sorry", axs.contains "sorryAx"),
      ("computable", kind == "def" && !isNoncomputable env n),
      ("doc", match doc with | some d => Json.str d.trimAscii.toString | none => Json.null),
      ("pos", pos)]
    lines := lines.push j.compress
  IO.FS.writeFile out (String.intercalate "\n" lines.toList ++ "\n")
