use crate::{ApiClient, ApiConfig};
use std::net::SocketAddr;
use warp::Filter;

pub struct ApiServer {
    #[allow(dead_code)]
    client: ApiClient,
    addr: SocketAddr,
}

impl ApiServer {
    pub fn new(config: ApiConfig, addr: SocketAddr) -> Self {
        Self {
            client: ApiClient::with_config(config),
            addr,
        }
    }

    pub async fn run(self) {
        let routes = self.routes();
        warp::serve(routes).run(self.addr).await;
    }

    fn routes(&self) -> impl Filter<Extract = impl warp::Reply, Error = warp::Rejection> + Clone {
        warp::path("health")
            .and(warp::get())
            .map(|| warp::reply::json(&serde_json::json!({"status": "ok"})))
    }
}