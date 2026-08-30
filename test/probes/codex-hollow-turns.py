#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Reproduce the population measurement behind the codex blocked reader.

Classifies every turn in every local codex rollout into the populations the
reader must separate, and lists the hollow turns with what follows each one.

Run it rather than trusting a number written down: the corpus changes, so a
tally in a document is stale the moment the next session ends. The claim the
reader rests on is the SHAPE of the separation -- that hollow turns are
distinguishable from ordinary completions, from turns that worked and closed
without a message, from compaction turns, and from interrupted aborts -- not
any particular count.

A hollow turn is a task_complete carrying no last_agent_message and no
time_to_first_token_ms, over a body holding nothing but the input that opened
the turn. The "next" column shows what follows each one: a codex turn only ever
begins from an input, so a following task_started is evidence that something was
sent, never that the window recovered on its own.
"""
import json,os,sys,collections
root=os.path.expanduser("~/.codex/sessions")
WORK_RESP={"reasoning","custom_tool_call","custom_tool_call_output","function_call",
           "function_call_output","local_shell_call","web_search_call"}
WORK_EV={"item_completed","patch_apply_end","web_search_end","sub_agent_activity",
         "agent_message","context_compacted"}
cls=collections.Counter(); hollows=[]
files=0
for dp,_,fns in os.walk(root):
    for fn in sorted(fns):
        if not fn.endswith(".jsonl"): continue
        path=os.path.join(dp,fn); files+=1
        recs=[]
        try:
            with open(path,errors="replace") as fh:
                for n,line in enumerate(fh,1):
                    line=line.strip()
                    if not line: continue
                    try: r=json.loads(line)
                    except Exception: continue
                    p=r.get("payload") if isinstance(r.get("payload"),dict) else {}
                    recs.append((n,r.get("type"),p.get("type"),p))
        except OSError: continue
        open_i=None; body=[]
        for idx,(n,rt,pt,p) in enumerate(recs):
            if pt=="task_started": open_i=idx; body=[]; continue
            if open_i is None: continue
            if pt=="task_aborted" or pt=="turn_aborted":
                cls["abort:"+str(p.get("reason"))]+=1; open_i=None; continue
            if pt=="task_complete":
                msg=p.get("last_agent_message"); ttft=p.get("time_to_first_token_ms")
                worked=any((b[2] in WORK_RESP) or (b[2] in WORK_EV) for b in body)
                compaction=any(b[2]=="context_compacted" for b in body)
                hollow_fields = (not msg) and (ttft is None)
                if compaction: cls["compaction"]+=1
                elif not hollow_fields: cls["ordinary"]+=1
                elif worked: cls["work-no-message"]+=1
                else:
                    cls["HOLLOW"]+=1
                    tail=[(recs[k][1],recs[k][2]) for k in range(idx+1,min(idx+4,len(recs)))]
                    hollows.append((path,recs[open_i][0],n,len(recs),p.get("duration_ms"),tail))
                open_i=None; continue
            body.append((n,rt,pt,p))
print("files:",files)
for k,v in cls.most_common(): print(f"  {k:28} {v}")
print("\nHOLLOW turns:",len(hollows))
for h in hollows:
    print(f"  {os.path.basename(h[0])[:46]} lines {h[1]}..{h[2]} of {h[3]} dur={h[4]} next={h[5]}")
