#!/usr/bin/env python3
import os
import json
import base64
import urllib.request
import urllib.error
import subprocess
import time
import sys

WORKSPACE_DIR = "/app/workspace"
OUT_DIR = os.path.join(WORKSPACE_DIR, "out")
IN_DIR = os.path.join(WORKSPACE_DIR, "in")
WORK_DIR = os.path.join(WORKSPACE_DIR, "work")
WWW_DIR = os.path.join(WORKSPACE_DIR, "www")

for d in [OUT_DIR, IN_DIR, WORK_DIR, WWW_DIR]:
    os.makedirs(d, exist_ok=True)

# Important to work in WORK_DIR so out/ isn't polluted by accident
os.chdir(WORK_DIR)

def build_system_prompt():
    import datetime
    name = os.environ.get("AGENT_NAME", "Agent")
    role = os.environ.get("AGENT_ROLE", "Assistant")
    personality = os.environ.get("PERSONALITY", "")
    current_time = datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
    
    personality_block = f"\nYour Personality: {personality}\n" if personality else ""
    
    script_dir = os.path.dirname(os.path.abspath(__file__))
    prompt_file = os.path.join(script_dir, "system_prompt.txt")
    
    try:
        with open(prompt_file, "r") as f:
            prompt_template = f.read()
    except FileNotFoundError:
        prompt_template = "You are {name}, an autonomous AI agent.\nYour Role: {role}"
    
    return prompt_template.format(
        name=name,
        role=role,
        personality_block=personality_block,
        current_time=current_time
    )

def extract_json_fields(response):
    import re
    json_match = re.search(r'\{[\s\S]*\}', response)
    if not json_match:
        return {}, None
    
    try:
        json_str = json_match.group()
        data = json.loads(json_str)
        return data, json_str
    except:
        return {}, None

def extract_command(response):
    data, _ = extract_json_fields(response)
    terminal = data.get("terminal", "")
    if terminal:
        return terminal, None
    return None, None

def extract_panel_actions(response):
    data, _ = extract_json_fields(response)
    panel_actions = data.get("panelActions", [])
    if isinstance(panel_actions, list):
        return panel_actions
    return []

def extract_user_id(response):
    data, _ = extract_json_fields(response)
    return data.get("userId", "")

def extract_message(response):
    data, _ = extract_json_fields(response)
    msg = data.get("message", "")
    if not msg:
        import re
        json_match = re.search(r'\{[\s\S]*\}', response)
        if json_match:
            return response[:json_match.start()].strip()
        return response.strip()
    return msg

def extract_action(response):
    data, _ = extract_json_fields(response)
    return data.get("action", "")

def is_dangerous(cmd):
    dangerous_tools = ["rm", "sudo", "su", "shutdown", "reboot", "nmap", "kill", "docker", "spawn_agent"]
    base = cmd.strip().split()[0] if cmd.strip() else ""
    for tool in dangerous_tools:
         if base == tool or base.startswith(tool):
             return True
    return False

def wait_for_approval():
    print("[HITL] Waiting for approval...", flush=True)
    lock_file = "/tmp/hermit_approval.lock"
    deny_file = "/tmp/hermit_deny.lock"
    waited = 0
    while waited < 600:
        if os.path.exists(lock_file):
            os.remove(lock_file)
            print("[HITL] Approved!", flush=True)
            return True
        if os.path.exists(deny_file):
            os.remove(deny_file)
            print("[HITL] Denied!", flush=True)
            return False
        time.sleep(1)
        waited += 1
    return False

def call_llm(messages):
    orchestrator_url = os.environ.get("ORCHESTRATOR_URL", "http://172.17.0.1:3000")
    agent_id = os.environ.get("AGENT_ID", "0")
    
    req_body = json.dumps({
        "messages": messages,
        "agentId": agent_id
    }).encode("utf-8")
    
    req = urllib.request.Request(f"{orchestrator_url}/api/internal/llm", data=req_body, headers={"Content-Type": "application/json"})
    
    try:
        with urllib.request.urlopen(req) as response:
            res_data = json.loads(response.read().decode("utf-8"))
            return res_data.get("output", "")
    except Exception as e:
        return f"Error communicating with Orchestrator Proxy: {str(e)}"

def main():
    user_msg = os.environ.get("USER_MSG", "")
    history_b64 = os.environ.get("HISTORY", "")
    hitl_enabled = os.environ.get("HITL_ENABLED", "false") == "true"
    
    history = []
    if history_b64:
        try:
            history = json.loads(base64.b64decode(history_b64).decode("utf-8"))
        except:
            pass

    # Note: RAG Memories are injected by the Orchestrator via the proxy, 
    # so we just need to send the standard system prompt.
    messages = [{"role": "system", "content": build_system_prompt()}]
    messages.extend(history)
    messages.append({"role": "user", "content": user_msg})
    
    max_iters = 5
    iters = 0
    
    while iters < max_iters:
        iters += 1
        response = call_llm(messages)
        
        # print COMMAND lines for visual streaming/debugging in logs
        for line in response.split("\n"):
            if line.strip().startswith("COMMAND:"):
                # We prefix with [INTERNAL] so we can filter if needed, 
                # but currently everything goes to the log/chat.
                # To keep chat clean, we could skip printing these if it's not the final response.
                pass
                
        messages.append({"role": "assistant", "content": response})
        
        cmd, _ = extract_command(response)
        if cmd:
            if is_dangerous(cmd) and hitl_enabled:
                print(f"[HITL] APPROVAL_REQUIRED: {cmd}", flush=True)
                if not wait_for_approval():
                    messages.append({"role": "user", "content": "ERROR: Command denied by user"})
                    continue
                print(f"[HITL] EXECUTING: {cmd}", flush=True)
            
            # Execute command
            try:
                result = subprocess.run(cmd, shell=True, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120)
                out = result.stdout
                if not out:
                    out = "Command executed successfully with no output."
                # Mark command output as internal - don't show to user
                messages.append({"role": "user", "content": f"[INTERNAL_COMMAND_OUTPUT]\n{out}"})
            except Exception as e:
                messages.append({"role": "user", "content": f"ERROR executing command: {str(e)}"})
        else:
            user_id = extract_user_id(response)
            message = extract_message(response)
            action = extract_action(response)
            
            json_fields, json_str = extract_json_fields(response)
            
            if json_fields:
                output_parts = []
                if user_id:
                    output_parts.append(f"userId:{user_id}")
                if message:
                    output_parts.append(f"message:{message}")
                if action:
                    output_parts.append(f"action:{action}")
                print(json.dumps({
                    "userId": user_id,
                    "message": message,
                    "action": action,
                    "terminal": "",
                    "panelActions": json_fields.get("panelActions", [])
                }), flush=True)
            else:
                print(message, flush=True)
            break

if __name__ == "__main__":
    main()
