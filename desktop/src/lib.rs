use std::io::{Read, Write};
use std::net::TcpStream;
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use tauri::{AppHandle, Manager, Url};
use tauri_plugin_shell::ShellExt;

const API_HOST: &str = "127.0.0.1";
const PORT_START: u16 = 42720;
const PORT_END: u16 = 42760;

#[derive(Clone)]
struct DesktopState {
    sidecar: Arc<Mutex<Option<Child>>>,
}

impl DesktopState {
    fn new() -> Self {
        Self {
            sidecar: Arc::new(Mutex::new(None)),
        }
    }

    fn set_child(&self, child: Child) {
        if let Ok(mut slot) = self.sidecar.lock() {
            *slot = Some(child);
        }
    }

    fn stop(&self) {
        if let Ok(mut slot) = self.sidecar.lock() {
            if let Some(mut child) = slot.take() {
                let _ = child.kill();
                let _ = child.wait();
            }
        }
    }
}

fn choose_port() -> Result<u16, String> {
    for port in PORT_START..=PORT_END {
        if std::net::TcpListener::bind((API_HOST, port)).is_ok() {
            return Ok(port);
        }
    }
    Err(format!("no available local port in {}-{}", PORT_START, PORT_END))
}

fn sidecar_command(app: &AppHandle) -> Result<Command, String> {
    let sidecar = app
        .shell()
        .sidecar("vola")
        .map_err(|err| format!("resolve Vola sidecar: {err}"))?;
    Ok(Command::from(sidecar))
}

fn wait_for_health(api_base: &str, timeout: Duration) -> Result<(), String> {
    let started = Instant::now();
    let health_path = "/api/health";
    while started.elapsed() < timeout {
        if local_http_request("GET", API_HOST, api_base, health_path, Duration::from_secs(2))
            .map(|response| response_has_status(&response, &[200]))
            .unwrap_or(false)
        {
            return Ok(());
        }
        std::thread::sleep(Duration::from_millis(250));
    }
    Err(format!(
        "Vola local service did not become healthy at {}{}",
        api_base.trim_end_matches('/'),
        health_path
    ))
}

fn bootstrap_owner_token(api_base: &str) -> Result<String, String> {
    let response = local_http_request(
        "POST",
        API_HOST,
        api_base,
        "/api/local/owner-token",
        Duration::from_secs(6),
    )?;
    if !response_has_status(&response, &[200, 201]) {
        return Err("local owner token request did not return 200 or 201".to_string());
    }
    let body = response
        .split_once("\r\n\r\n")
        .map(|(_, body)| body)
        .unwrap_or_default();
    let token = extract_json_string(body, "token").unwrap_or_default();
    if token.is_empty() {
        return Err("local owner token response did not include a token".to_string());
    }
    Ok(token)
}

fn response_has_status(response: &str, expected: &[u16]) -> bool {
    let Some(status) = response
        .lines()
        .next()
        .and_then(|line| line.split_whitespace().nth(1))
        .and_then(|code| code.parse::<u16>().ok())
    else {
        return false;
    };
    expected.contains(&status)
}

fn local_http_request(
    method: &str,
    host: &str,
    api_base: &str,
    path: &str,
    timeout: Duration,
) -> Result<String, String> {
    let port = api_base
        .rsplit_once(':')
        .and_then(|(_, port)| port.parse::<u16>().ok())
        .ok_or_else(|| format!("parse local service port from {api_base}"))?;
    let mut stream = TcpStream::connect((host, port))
        .map_err(|err| format!("connect local service {host}:{port}: {err}"))?;
    stream
        .set_read_timeout(Some(timeout))
        .map_err(|err| format!("set local read timeout: {err}"))?;
    stream
        .set_write_timeout(Some(timeout))
        .map_err(|err| format!("set local write timeout: {err}"))?;
    let request = format!(
        "{method} {path} HTTP/1.1\r\nHost: {host}:{port}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
    );
    stream
        .write_all(request.as_bytes())
        .map_err(|err| format!("write local request: {err}"))?;
    let mut response = String::new();
    stream
        .read_to_string(&mut response)
        .map_err(|err| format!("read local response: {err}"))?;
    Ok(response)
}

fn extract_json_string(body: &str, key: &str) -> Option<String> {
    let needle = format!("\"{key}\"");
    let after_key = body.split(&needle).nth(1)?;
    let after_colon = after_key.split_once(':')?.1.trim_start();
    let value = after_colon.strip_prefix('"')?;
    let end = value.find('"')?;
    Some(value[..end].to_string())
}

fn start_local_service(app: &AppHandle, state: &DesktopState) -> Result<(String, String), String> {
    let port = choose_port()?;
    let listen_addr = format!("{API_HOST}:{port}");
    let api_base = format!("http://{listen_addr}");

    let mut cmd = sidecar_command(app)?;
    cmd.args([
        "server",
        "--local-mode",
        "--listen",
        &listen_addr,
        "--storage",
        "sqlite",
        "--public-base-url",
        &api_base,
    ])
    .env("VOLA_LOCAL_MODE", "1")
    .env("PORT", port.to_string())
    .env("CORS_ORIGINS", &api_base)
    .stdout(Stdio::null())
    .stderr(Stdio::null());

    let child = cmd
        .spawn()
        .map_err(|err| format!("start Vola local service: {err}"))?;
    state.set_child(child);
    wait_for_health(&api_base, Duration::from_secs(20))?;
    let token = bootstrap_owner_token(&api_base)?;
    Ok((api_base, token))
}

fn load_desktop_dashboard(app: &AppHandle, api_base: &str, token: &str) -> Result<(), String> {
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "main window was not created".to_string())?;
    let target = format!(
        "{}/?local_token={}&desktop=1",
        api_base.trim_end_matches('/'),
        token
    );
    let url = Url::parse(&target).map_err(|err| format!("parse local dashboard URL: {err}"))?;
    window
        .navigate(url)
        .map_err(|err| format!("open local dashboard: {err}"))?;
    Ok(())
}

pub fn run() {
    let state = DesktopState::new();
    let window_stop_state = state.clone();
    let exit_stop_state = state.clone();

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_shell::init())
        .setup(move |app| {
            let handle = app.handle().clone();
            let state = state.clone();
            match start_local_service(&handle, &state)
                .and_then(|(api_base, token)| load_desktop_dashboard(&handle, &api_base, &token))
            {
                Ok(()) => Ok(()),
                Err(err) => Err(Box::<dyn std::error::Error>::from(err)),
            }
        })
        .on_window_event(move |_window, event| {
            if matches!(event, tauri::WindowEvent::Destroyed) {
                window_stop_state.stop();
            }
        })
        .build(tauri::generate_context!())
        .expect("error while building Vola desktop application")
        .run(move |_app, event| {
            if matches!(event, tauri::RunEvent::ExitRequested { .. }) {
                exit_stop_state.stop();
            }
        });
}
