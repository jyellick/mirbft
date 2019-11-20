import React, { Component } from 'react';

import Card from 'react-bootstrap/Card'
import Form from 'react-bootstrap/Form'
import InputGroup from 'react-bootstrap/InputGroup'
import Container from 'react-bootstrap/Container'
import Row from 'react-bootstrap/Row'
import Col from 'react-bootstrap/Col'
import Table from 'react-bootstrap/Table'
import Button from 'react-bootstrap/Button'

class SequenceHeaders extends Component {
  render() {
    return <thead>
      <tr>
        <th colSpan="2" key="unused"></th>
        {[...Array(this.props.highWatermark - this.props.lowWatermark + 1).keys()].map((i) => {
          return <th key={"header-"+i}>{i+this.props.lowWatermark}</th>;
        })}
      </tr>
    </thead>
  }
}

class SequenceStatusRow extends Component {
  render() {
    let count = 0;

    return <tr>
      {(() => {if(this.props.bucket.ID === 0) {
        return <td rowSpan={this.props.numBuckets} style={{verticalAlign:"middle"}}>
          <Button onClick={this.props.onToggle}>Node-{this.props.nodeID} State Machine</Button>
        </td>
      }})()}

      <td>Bucket-{this.props.bucket.ID}</td>

      {[...Array(this.props.offset).keys()].map((i) => {
          return <td key={"offset-"+i} style={{background:"black"}}></td>;
      })}

      {
        this.props.bucket.Sequences.map((sequence) => {
        count++;
        if(sequence === 0){
          return <td key={count}></td>;
        } else if(sequence === 1){
          return <td  key={count} style={{background:"yellow"}}>Q</td>;
        } else if(sequence === 2){
          return <td  key={count} style={{background:"yellow"}}>D</td>;
        } else if(sequence === 3){
          return <td  key={count} style={{background:"red"}}>I</td>;
        } else if(sequence === 4){
          return <td  key={count} style={{background:"yellow"}}>V</td>;
        } else if(sequence === 5){
          return <td  key={count} style={{background:"yellow"}}>P</td>;
        } else if(sequence === 6){
          return <td  key={count} style={{background:"green"}}>C</td>;
        } else {
          return <td key={count}>?</td>
        }
      })}

      {[...Array(this.props.padding).keys()].map((i) => {
        return <td key={"padding-"+i} style={{background:"gray"}}></td>
      })}
    </tr>
  }
}

class SequenceMsgRow extends Component {
  render() {
    return <tr>
      {(() => {
        if(this.props.bucket.BucketID === 0) {
          return <td rowSpan={this.props.numBuckets} style={{verticalAlign:"middle"}}>
            Node-{this.props.nodeID}
          </td>
        }
      })()}

      <td>Bucket-{this.props.bucket.BucketID}</td>

      {[...Array(this.props.offset).keys()].map((i) => {
          return <td key={"offset-"+i} style={{background:"black"}}></td>;
      })}

      {[...Array(this.props.highWatermark - this.props.lowWatermark + 1).keys()].map((i) => {
        let bucket = this.props.bucket;
        let seq = i + this.props.lowWatermark;
        if(bucket.LastCheckpoint === seq){
          return <td key={"seq-"+seq}>X</td>;
        } else if(bucket.LastCommit === seq){
           return <td key={"seq-"+seq}>C</td>;
        } else if(bucket.LastPrepare === seq){
          return <td key={"seq-"+seq}>P</td>;
        } else {
          return <td key={"seq-"+seq}></td>;
        }
      })}

      {[...Array(this.props.padding).keys()].map((i) => {
        return <td key={"padding-"+i} style={{background:"gray"}}></td>
      })}
    </tr>
  }
}

class SequenceCheckpointRow extends Component {
  render() {
    let skipped = 0;
    return <tr>
      <td></td>
      <td>Checkpoints</td>
      {[...Array(this.props.offset).keys()].map((i) => {
          return <td key={"offset-"+i} style={{background:"black"}}></td>;
      })}

      {[...Array(this.props.highWatermark - this.props.lowWatermark + 1).keys()].map((i) => {
        let seq = i + this.props.lowWatermark+this.props.offset;
        let checkpoint = this.props.checkpoints.find((checkpoint) => {
          return checkpoint.SeqNo === seq
        })
        if(checkpoint) {
          let colSpan = skipped+1;
          skipped = 0;
          if(checkpoint.LocalAgreement && checkpoint.NetQuorum) {
            return <td colSpan={colSpan} key={"seq-"+seq} style={{background:"green",textAlign:"center"}}>Local+Network</td>;
          } else if(checkpoint.NetQuorum) {
            return <td colSpan={colSpan} key={"seq-"+seq} style={{background:"yellow",textAlign:"center"}}>Network</td>;
          } else {
            return <td colSpan={colSpan} key={"seq-"+seq} style={{textAlign:"center"}}>Pending {checkpoint.PendingCommits}</td>;
          }
        } else {
          skipped++
          return null
        }
      })}

      {[...Array(this.props.padding).keys()].map((i) => {
        return <td key={"padding-"+i} style={{background:"gray"}}></td>
      })}
    </tr>
  }
}

class SequenceMsgBody extends Component {
  render() {
    if(this.props.collapsed) {
      return null
    }
    return this.props.nodes.map((node) => {
        return <tbody key={"node-msg-tbody-"+this.props.NodeID+"-"+node.ID} style={{border:"solid black 3px"}}>
          { node.BucketStatuses.map((bucket) => {
          return <SequenceMsgRow key={"node-msg-row-"+this.props.nodeID+"-"+node.ID+"-"+bucket.BucketID} bucket={bucket} offset={this.props.offset} padding={this.props.padding} lowWatermark={this.props.lowWatermark} highWatermark={this.props.highWatermark} numBuckets={node.BucketStatuses.length} nodeID={node.ID}/>
        })}
     </tbody>
    })
  }
}

class SequenceStatusBody extends Component {
  render() {
    return <tbody style={{border:"solid black 3px"}}>
      {this.props.buckets.map((bucket) => {
        return <SequenceStatusRow key={"node-"+this.props.nodeID+"-"+bucket.ID} bucket={bucket} offset={this.props.offset} padding={this.props.padding} numBuckets={this.props.buckets.length} nodeID={this.props.nodeID} onToggle={this.props.onToggle}/>
      })}
      <SequenceCheckpointRow offset={this.props.offset} padding={this.props.padding} lowWatermark={this.props.lowWatermark} highWatermark={this.props.highWatermark} checkpoints={this.props.checkpoints}/>
    </tbody>
  }
}

class StatusTable extends Component {
  constructor(props) {
    super(props);
    this.state = {
      collapsed: true
    }
    this.toggleCollapse = this.toggleCollapse.bind(this)
  }

  toggleCollapse() {
    this.setState({
      collapsed: !this.state.collapsed
    })
  }

  render() {
    let lowWatermark = Math.min(...(this.props.nodes.map((i) => {return i.stateMachine.LowWatermark})))
    let highWatermark = Math.max(...(this.props.nodes.map((i) => {return i.stateMachine.HighWatermark})))
    if(highWatermark <= lowWatermark) {
      return null
    }

    return <Table bordered size="sm">
       <SequenceHeaders lowWatermark={lowWatermark} highWatermark={highWatermark} />
       {this.props.nodes.map((node) => {
         let offset = node.stateMachine.LowWatermark - lowWatermark;
         let padding = highWatermark - node.stateMachine.HighWatermark;
         return <React.Fragment key={"node-fragment"+node.ID}>
           <SequenceStatusBody key={"node-status-"+node.ID} nodeID={node.ID} buckets={node.stateMachine.Buckets} checkpoints={node.stateMachine.Checkpoints} offset={offset} padding={padding} onToggle={this.toggleCollapse} lowWatermark={lowWatermark} highWatermark={highWatermark}/>
           <SequenceMsgBody collapsed={this.state.collapsed} key={"node-msg-"+node.ID} nodeID={node.ID} offset={offset} padding={padding} nodes={node.stateMachine.Nodes} lowWatermark={node.stateMachine.LowWatermark} highWatermark={node.stateMachine.HighWatermark}/>
          </React.Fragment>
       })}
    </Table>
  }
}

class NodeControl extends Component {

  constructor(props) {
    super(props);
    this.state = {
      automatic: true
    }
    this.processing = false
    this.switchProcessing = this.switchProcessing.bind(this)
  }
  

  handleProcess(e) {
    if(this.processing === true) {
      return
    }
    this.processing = true;
    let delay = this.state.automatic
    if(this.state.automatic === "manual")  {
      delay = 0
    }

    setTimeout(() => {
      fetch("/node/"+this.props.node+"/process")
        .then(() => {
          this.processing = false
          this.props.update();
        })
        .catch(function(error) {
          console.log(error);
          alert(error);
        });
    }, delay)
  }

  handlePropose(e) {
    e.preventDefault()
    let proposals = Number(e.target.elements[1].value)
    for(let i = 1; i <= proposals; i++) {
      fetch("/node/"+this.props.node+"/propose", {
        method:"POST",
        body: JSON.stringify([...Array(10*1024).keys()].map(() => {return Math.random();}))
      }).then(() => {
          if(i === proposals) {
            this.props.update();
          }
        })
        .catch(function(error) {
          console.log(error);
          alert(error);
        });
    }
  }

  switchProcessing(e) {
    this.setState({automatic: e.target.value})
  }


  render() {
    if(this.props.actions.total > 0 && this.state.automatic !== "manual") {
      this.handleProcess(null)
    }

    return <Card>
      <Card.Body>
        <Card.Title>Node {this.props.node}</Card.Title>
          <Card.Text>State: {this.props.log.LastBytes}</Card.Text>
          <Card.Text>Committed: {Math.floor(this.props.log.TotalBytes/1024)} kb</Card.Text>
          <Table size="sm">
              <thead><tr><th>Action</th><th>Outstanding</th></tr></thead>
              <tbody>
                <tr><td>Broadcasts</td><td>{this.props.actions.broadcast}</td></tr>
                <tr><td>Unicasts</td><td>{this.props.actions.unicast}</td></tr>
                <tr><td>Preprocess</td><td>{this.props.actions.preprocess}</td></tr>
                <tr><td>Digest</td><td>{this.props.actions.digest}</td></tr>
                <tr><td>Validate</td><td>{this.props.actions.validate}</td></tr>
                <tr><td>Commit</td><td>{this.props.actions.commit}</td></tr>
                <tr><td>Checkpoint</td><td>{this.props.actions.checkpoint}</td></tr>
                <tr style={{fontWeight:"bold"}}><td>Total</td><td>{this.props.actions.total}</td></tr>
              </tbody>
          </Table>
        <Form onSubmit={(e) => this.handleProcess(e)}>
          <Form.Label id='auto'>Processing </Form.Label>
          <Form.Control as="select" onClick={(e) => {this.switchProcessing(e)}}>
            <option value="0">Automatic (immediate)</option>
            <option value="500">Automatic (500ms delay)</option>
            <option value="1000">Automatic (1000ms delay)</option>
            <option value="2000">Automatic (2000ms delay)</option>
            <option value="manual">Manual</option>
          </Form.Control>
          <InputGroup>
            <Button className="m-2" disabled={this.props.actions.total === 0 || this.state.automatic === true} onClick={(e) => {this.handleProcess(e)}}> Manual Step </Button>
          </InputGroup>
        </Form>
        <Form onSubmit={(e) => this.handlePropose(e)}>
            <InputGroup>
              <Button type="submit" className="m-2"> Propose </Button>
              <Form.Control defaultValue="1" className="m-2"/>
            </InputGroup>
        </Form>
        </Card.Body>
      </Card>
  }
}

class App extends Component {
  constructor(props) {
    super(props);
    this.refreshing = false;
    this.state = {
      nodes: [],
      refreshing: false
    };
  
    this.updateStatus = this.updateStatus.bind(this);
    this.updateStatus(0);
  }

  updateStatus() {
    if( this.refreshing === true) {
      return
    }
    this.refreshing= true
    fetch("/status")
      .then(response => response.json())
      .then(nodeStatuses => {
        this.setState({
          nodes: nodeStatuses
        });
        this.refreshing = false
      })
      .catch(function(error) {
        console.log(error);
        alert(error);
      });
  }

  render() {
    return (
      <div className="App">
        <header className="App-header">
          <Container className="m-1">
            <Row><Col><h1>MirBFT Status</h1></Col></Row>
            <Row>
              {this.state.nodes.map((node) => {
                return <Col key={"node-col-top-"+node.ID}><NodeControl key={"node-control-top-"+node.ID} node={node.ID} actions={node.actions} log={node.log} update={this.updateStatus}/></Col>
              })}
            </Row>
            <Row><Col> <StatusTable nodes={this.state.nodes}/> </Col></Row>
          </Container>
        </header>
      </div>
    );
  }
}

export default App;
